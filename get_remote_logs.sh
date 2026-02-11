#!/bin/bash
log show --style syslog --last 5m --predicate 'sender contains "kernel" or sender contains "driverkit" or process == "virtual-fido"' > /tmp/vm_debug.log
grep -i -E "VirtualUSBDriver|virtual-fido|AMFI|signature|entitlement" /tmp/vm_debug.log
