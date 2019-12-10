# qovery-cli

Qovery helps tech companies to accelerate and scale application development cycle with zero infrastructure management investment.

### Documentation
See our complete documentation [here](https://docs.qovery.com)

# Installation
### Windows
Windows installer [here](https://google.com)

### Linux
#### Debian/Ubuntu
```
apt install qovery
```

#### Fedora/RHEL
```
yum install qovery
```

#### Mac OS
```
brew install qovery
```

# Usage
### Before getting started
- Create an account on [qovery.com](https://qovery.com)
- Sign in through Qovery CLI (see below)

## Basic usage

#### Authentication
```
qovery auth

Authentication code: *******

```
Copy/paste the authentication code that you will receive from your browser.

#### Start using Qovery within your application
```
qovery init
```

## Advanced usage
Once Qovery is linked to your Github, Bitbucket or Gitlab account AND
once you have init a `.qovery.yml` file, all the following tasks are done automatically. 

* Create, update, delete an [application](https://docs.qovery.com/services/applications)
* Create, update, delete a [database](https://docs.qovery.com/services/databases)
* Create, update, delete a [broker](https://docs.qovery.com/services/brokers)
* Create, update, delete a [storage](https://docs.qovery.com/services/storage)

### Project
#### List all your projects
```
qovery project list
```

### Application
#### List all environments
```
qovery environment list
```
or
```
qovery environment list -p <project_name>
```

#### Show environment status
```
qovery status
```
or
```
qovery environment status
```
or
```
qovery environment status -p <project_name>
```

#### List all applications from environment
```
qovery application list
```
or
```
qovery application list -p <project_name> -e <environment_name>
```

#### List all routes from environment
```
qovery route list
```
or
```
qovery route list -p <project_name> -e <environment_name>
```

#### List all databases from environment
```
qovery database list
```
or
```
qovery database list -p <project_name> -e <environment_name>
```

#### List all brokers from environment
```
qovery broker list -p <project_name> -e <environment_name>
```

#### List all storage from environment
```
qovery storage list -p <project_name> -e <environment_name>
```
