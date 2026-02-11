#!/bin/bash
log show --style syslog --last 10m --predicate 'sender contains "kernel" or process == "sysextd" or process == "taskgated"' > /tmp/debug_full.log
echo "--- GREP BULWARK ---"
grep -i "bulwark" /tmp/debug_full.log
echo "--- GREP VIRTUALUSB ---"
grep -i "VirtualUSBDriver" /tmp/debug_full.log
echo "--- GREP AMFI REJECTION ---"
grep -i "AMFI" /tmp/debug_full.log | grep -i "deny"
