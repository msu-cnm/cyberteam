

# Firewalld

1. List firewall rules for the default zone
   `firewall-cmd --list-all`

2. If interface isn't listed, find which zone interface is in or use
   `firewall-cmd --get-active-zones`

3. If needed, rerun your rule list command
   `firewall-cmd --zone=WHATEVER --list-all`

4. Examine services/ports that are allowed & compare against what should be allowed

5. Remove/Add any ports/services as needed
   ```bash
   firewall-cmd [--zone=WHATEVER] --add-service=NAME
   firewall-cmd [--zone=WHATEVER] --add-port=NUMBER/PROTOCOL
   
   firewall-cmd [--zone=WHATEVER] --remove-service=NAME
   firewall-cmd [--zone=WHATEVER] --remove-port=NUMBER/PROTOCOL
   ```

6. When finished, save config
   `firewall-cmd --runtime-to-permanent`

# HTTP Security

1. Check out `/etc/httpd/conf/httpd.conf`

2. Look for security holes
   ```bash
   # Make sure user/group are set to no-privilege user like apache (You can check /etc/passwd for a service account to use or make one)
   User apache
   Group apache
   
   # NOTE: If you need to create a no privilege user
   # useradd -M USERNAME
   # usermod -L USERNAME
   
   <Directory "SOMETHING">
   	AllowOverride None ## This should always be none
   </Directory>
   
   
   <Directory /PATH/TO/WEBDIR/wp-admin>
       # allow access from one IP and an additional IP range,
       # and block everything else
       Require ip 1.2.3.4
       Require ip 192.168.0.0/24
   </Directory>
   ```

   
