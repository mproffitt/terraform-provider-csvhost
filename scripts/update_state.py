#!/usr/bin/python3
import os
import shutil
import json
import pandas as pd
from datetime import datetime
from datetime import timedelta

class TerraformStateUpdate(object):
    _state_file = 'terraform.tfstate'
    _backup_file = 'terraform.tfstate.py-backup'
    _state = None
    _new_state = None

    def __init__(self, csvfile='csv/twickenhamlabs.csv'):
        if not os.path.exists(self._state_file):
            raise EOFError('backup file has not yet been created')
        # copy to a backup file first. We're going to read from this one
        if not self.backup_state():
            raise RuntimeError('Failed to copy state file')
        self.load_state()
        self.load_csv(csvfile)

    def load_state(self):
        """
        Loads the state file into memory
        """
        try:
            with open(self._backup_file) as data:
                self._state = json.loads(data.read())
                data.close()
        except Exception as exception:
            print('[ERROR] - Failed to load state file into memory')
            print('[ERROR] - Exception was')
            print(str(exception))
            raise RuntimeError('Failed to load state file')

    def load_csv(self, csvfile):
        """
        Loads the CSV file into a pandas dataframe

        :param csvfile: string - full path to the csvfile
        """
        # Use pandas to read the csv and get rid of any null entries
        if not os.path.exists(csvfile):
            raise RuntimeError('Failed to find CSV file at {csvfile}'.format(csvfile=csvfile))

        try:
            self._csv = pd.read_csv(csvfile)
            self._csv.dropna(how="all", inplace=True)
        except Exception as exception:
            print('[ERROR] - failed to load csv file into dataframe')
            print('[ERROR] - exception was')
            print(str(exception))
            raise

    def backup_state(self):
        """
        Creates a backup of the current state file which will be used for this run

        :return: boolean
        """
        if not os.path.exists(self._backup_file):
            try:
                shutil.copyfile(self._state_file, self._backup_file)
                return True
            except IOError as e:
                print('[ERROR] - failed to create state file backup. Please check permissions and try again')
                return False
        print('[ERROR] - backup state file already exists. Did a previous execution error?')
        print('[ERROR] - if not, please delete this file and try again')
        return False

    def _get_key(self, resource_key):
        """
        Gets the resource key by stripping the int off the end of the current key

        :param resource_key: string
        :return: string
        """
        try:
            if isinstance(int(resource_key.split('.')[-1]), int):
                return '.'.join(resource_key.split('.')[:-1])
        except ValueError:
            pass
        return resource_key

    def get_machine_type(self, key):
        """
        Gets the type (size) of machine based on the template name

        :param key: string
        :return tuple: (machine_name, data_provider)
        """
        machine_type = key.split('.')[-1].replace('-standard', '').split('-')
        machine_type[1] = machine_type[1].upper() # 0 = OS, 1 = machine size [optional 2 = on/off the domain]
        machine_type = ''.join(machine_type)
        return machine_type, '{0}.{1}'.format('data.esscsvhost', machine_type)

    def update(self):
        print('====================================================================================')
        print('== Updating state file')
        for module_index,  module in enumerate(self._state['modules']):
            resources = module['resources']
            if len(module['path']) < 2:
                continue
            elif len(resources) == 0:
                continue

            replacements = {}
            new_resource = {}

            # first, grab all the data elements
            keys = list(resources.keys())
            for i in range(0, len(keys)):
                if keys[i].startswith('data.'):
                    replacements[keys[i]] = resources[keys[i]]
                    del resources[keys[i]]

            path = '/'.join(module['path'][2:])
            filtered = self._csv[self._csv['vapp'].str.strip('/').str.endswith(path)]
            if len(filtered) == 0:
                continue

            today = datetime.today()
            print('== Looking at module: {module}'.format(module=path))
            for current in resources.keys():
                key = self._get_key(current)
                item_index = None
                resource = resources[current]
                vapp_id = resource['primary']['id'].split('/')

                machine_type, data_provider = self.get_machine_type(key)
                size_filtered = filtered[filtered['template'] == machine_type]
                if data_provider not in resource['depends_on']:
                    resource['depends_on'].append(data_provider)

                try:
                    _ = new_resource[key]
                except:
                    new_resource[key] = {'stay': [], 'move': []}
                    new_resource[key]['stay'] = [None for _ in range(len(size_filtered))]

                added = False
                host_id = '/'.join(vapp_id[3:])
                counter = 0
                for _, host in size_filtered.iterrows():
                    expiry_date = None
                    if pd.isnull(host.expires):
                        expiry_date = datetime.today() + timedelta(days=365)
                    else:
                        try:
                            expiry_date = datetime.strptime(host.expires, '%Y-%m-%d')
                        except ValueError:
                            try:
                                expiry_date = datetime.strptime(host.expires, '%d/%m/%Y')
                            except ValueError:
                                raise ValueError(
                                    '[ERROR] - host {0} has invalid date format for expiry.\n' +
                                    '          Format should be \'YYYY-MM-DD\' or \'DD/MM/YYYY\', received {1}'.format(
                                        host.name,
                                        host.expires
                                    )
                                )


                    if host_id == '{0}/{1}'.format(host.vapp, host.hostname):
                        if ((expiry_date - today).days * 24) < -168: # expired 7 days prior
                            break
                        new_resource[key]['stay'][counter] = resource
                        added = True
                        break
                    counter += 1
                if not added:
                    new_resource[key]['move'].append(resource)

            for key in new_resource.keys():
                counter = 0
                for resource in new_resource[key]['stay']:
                    if resource is None:
                        continue
                    replacements['{key}.{index}'.format(key=key, index=counter)] = resource
                    print('[STAY] == {0} == {1}'.format(resource['primary']['attributes']['name'], '{key}.{index}'.format(key=key, index=counter)))
                    counter += 1
                for resource in new_resource[key]['move']:
                    replacements['{key}.{index}'.format(key=key, index=counter)] = resource
                    print('[DELETE] == {0} == {1}'.format(resource['primary']['attributes']['name'], '{key}.{index}'.format(key=key, index=counter)))
                    counter += 1
            print('====================================================================================')

            self._state['modules'][module_index]['resources'] = replacements

    def write(self):
        """
        Writes the state file out to source defined at self._state_file

        :return bool:
        """
        try:
            with open(self._state_file, 'w') as outfile:
                json.dump(self._state, outfile, sort_keys=True, indent='    ')
        except IOError as exception:
            print(
                '[ERROR] - Failed to update state file \'{statefile}\' - Nothing has been changed'.format(
                    statefile=self._state_file
                )
            )
            print('[ERROR] - Reason was: {exception}'.format(exception=str(exception)))
            return False
        return True

    def cleanup(self):
        if os.path.exists(self._backup_file):
            os.remove(self._backup_file)

try:
    terraform = TerraformStateUpdate()
    terraform.update()
    if terraform.write():
        terraform.cleanup()
    print('[INFO] - State file has been updated - please execute \'terraform plan\'')
except ValueError as exception:
    print(exception)
    exit(1)
except EOFError as exception:
    print('[INFO] - No state file exists. Presuming first run')
except Exception as exception:
    print('-------------------------------------------------------------')
    print('[ERROR] - Something went wrong whilst updating the state file')
    print('[ERROR] - The exception provided was:')
    print('[ERROR] - {exception}'.format(exception=str(exception)))
    exit(1)
