#!/bin/bash 


for x in radvd id systemctl ip sysctl dhclient ; do 
	which $x >/dev/null || { 
		echo "We need $x; exiting.."
		exit 2111
	} 
done

conf="/etc/ipv6thingy.conf"
if [ -s "$conf" ] ; then
	:
else
	echo "Please setup your $conf first.."
	exit 331
fi

if id -un |grep -q root ; then
	:
else
	echo "Need to be root to run $0"
	exit 313
fi

RC="1"
# let's set it up now.
	cp -v ipv6thingy.service /etc/systemd/system/ && \
	systemctl enable ipv6thingy.service && \
	cp -v bin/ipv6thingy-$(uname -m) /usr/local/bin/ipv6thingy && \
	cp -v ipv6hook /etc/dhcp/dhclient-exit-hooks.d/  && \
	chmod -v 755 /usr/local/bin/ipv6thingy /etc/dhcp/dhclient-exit-hooks.d/ipv6hook 
RC=$?

if [ "$RC" -eq 0 ]; 
then
	systemctl start ipv6thingy 
fi
	
