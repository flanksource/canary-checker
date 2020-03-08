# LDAP

This check will:

- bind using provided user/password to the ldap host. Supports ldap/ldaps protocols.
- search an object type in the provided bind DN.s

```yaml
ldap:
  - host: ldap://127.0.0.1:10389
    username: uid=admin,ou=system
    password: secret
    bindDN: ou=users,dc=example,dc=com
    userSearch: "(&(objectClass=organizationalPerson))"
  - host: ldap://127.0.0.1:10389
    username: uid=admin,ou=system
    password: secret
    bindDN: ou=groups,dc=example,dc=com
    userSearch: "(&(objectClass=groupOfNames))"
```