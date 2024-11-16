## Firewalld

### Setup

```bash
# install firewalld
apt install firewalld 
# enable service
systemctl enable firewalld.service 
# start service
systemctl start firewalld.service 
# get service status
systemctl status firewalld 
```



### List Rules and Available Services

```bash
# List rules
firewall-cmd --list-all

# List services
firewall-cmd --get-services

```



### Add / Delete Rules

```bash
# Allow a service
firewall-cmd --zone=public --add-service=<service> --permanent

# Block a service (Delete an existing rule)
firewall-cmd --zone=public --remove-service=<service> --permanent

# Allow a port
firewall-cmd --zone=public --add-port=<port>/<protocol> --permanent
# Example:  firewall-cmd --zone=public --add-port=80/tcp

# Block a port (Delete an existing rule)
firewall-cmd --zone=public --remove-port=<port>/<protocol> --permanent

```



### Reload Firewall (Apply changes)

```bash
# restart service (do this after changing rules)
systemctl restart firewalld.service 
```



### (Shouldn't need this, but just in case)

```bash
# stop the service
systemctl stop firewalld.service 
# dissable the service
systemctl dissable firewalld.service 
```

