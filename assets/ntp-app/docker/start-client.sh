while ! nslookup ntp; do
    echo "Failed to resolve service name"
    sleep 1
done

echo "Successfully resolved service name, starting ntpd"

/usr/sbin/ntpd -n -c /etc/ntp/client.conf
