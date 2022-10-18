## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/ldap.svg' style='height: 32px'/> LDAP

The LDAP check will:

* bind using provided user/password to the ldap host. Supports ldap/ldaps protocols.
* search an object type in the provided bind DN.s

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: ldap-check
     spec:
       interval: 30
       ldap:
         - host: ldap://apacheds.ldap.svc:10389
           auth:
             username:
               # value: uid=admin,ou=system 
               valueFrom: 
                 secretKeyRef:
                   name: ldap-credentials
                   key: USERNAME
             password: 
               valueFrom: 
                 secretKeyRef:
                   name: ldap-credentials
                   key: PASSWORD
           bindDN: ou=users,dc=example,dc=com
           userSearch: "(&(objectClass=organizationalPerson))"
         - host: ldap://apacheds.ldap.svc:10389
           auth:
             username:
               # value: uid=admin,ou=system 
               valueFrom: 
                 secretKeyRef:
                   name: ldap-credentials
                   key: USERNAME
             password:
               valueFrom: 
                 secretKeyRef:
                   name: ldap-credentials
                   key: PASSWORD
           bindDN: ou=groups,dc=example,dc=com
           userSearch: "(&(objectClass=groupOfNames))"
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **auth** |  | *[Authentication](#authentication) | Yes |
| **bindDN** |  | string | Yes |
| description | Description for the check | string |  |
| **host** |  | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| skipTLSVerify |  | bool |  |
| userSearch |  | string |  |