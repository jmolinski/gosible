The constants package contains all the _constants_ used in the application. Constants do not depend on config files or base definitions for config variables.

In Ansible, constants are mixed with config variables, and both reside in the constants module.

Keep in mind that it means that some names of "constants" from Ansible that you would expect to be defined in constants package is defined in config package instead.
