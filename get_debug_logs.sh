#!/bin/bash
/usr/bin/log show --style syslog --last 10m --predicate 'process == "kernel" OR process == "sysextd" OR sender contains "VirtualUSBDriver" OR process == "taskgated" OR process == "amfid"' > ~/debug_log.txt
