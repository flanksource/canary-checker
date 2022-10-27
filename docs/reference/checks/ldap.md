## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/ldap.svg' style='height: 32px'/> LDAP

The LDAP check:

* Binds using the provided username and password to the LDAP host. It supports LDAP/LDAPS protocols.
* Searches an object type in the provided `bindDN`.

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
| **auth** | username and password value, configMapKeyRef or SecretKeyRef for LDAP server | *[Authentication](#authentication) | Yes |
| **bindDN** | BindDN to use in query | string | Yes |
| description | Description for the check | string |  |
| **host** | URL of LDAP server to be qeuried | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
| skipTLSVerify | Skip check of LDAP server TLS certificates | bool |  |
| userSearch | UserSearch to use in query | string |  |

---
# Scheme Reference
## Authentication



| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| **password** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
| **username** |  | [kommons.EnvVar](https://pkg.go.dev/github.com/flanksource/kommons#EnvVar) | Yes |
