#!/bin/bash
ip link set dev eth0 down
ip addr flush dev eth0
ip addr add 192.168.1.100/24 dev eth0
ip link set dev eth0 up
ip route add default via 192.168.1.1
