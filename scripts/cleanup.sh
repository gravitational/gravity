#!/usr/bin/env bash
echo "Destroying virtual machines..."
for name in $(virsh list --name --all | grep "dev.local"); do virsh destroy $name; virsh undefine $name; done

echo "Deleting unused images..."
sudo find /var/lib/gravity/images -name "*dev.local*" -exec rm {} \;

echo "Removing test etcd container..."
docker stop testetcd0 && docker rm -f testetcd0

echo "Wiping out system directories..."
rm -rf /var/lib/gravity/opscenter/*
rm -rf /var/lib/gravity/local/*
rm -rf /var/lib/teleport/*
sudo rm -rf /var/lib/gravity/etcd/*

echo "Done!"
