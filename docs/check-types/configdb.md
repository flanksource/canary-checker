## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/configdb.svg' style='height: 32px'/> ConfigDB

ConfigDB check will connect to the specified host; run the specified query and return the result:

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: configdb-check
     spec:
       interval: 30
       configDB:
         - name: ConfigDB Check
           host: 192.168.1.5
           authentication:
             username: 
               valueFrom: 
               secretKeyRef:
                 name: configdb-credentials
                 key: USERNAME 
             password: 
               valueFrom: 
               secretKeyRef:
                 name: configdb-credentials
                 key: PASSWORD 
           query: "SELECT 1"
     ```
           

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| authentication |  | [Authentication](#authentication) |  |
| description | Description for the check | string |  |
| display |  | [Template](#template) |  |
| **host** |  | string | Yes |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| labels | Labels for the check | Labels |  |
| **name** | Name of the check | string | Yes |
| **query** |  | string | Yes |
| test |  | [Template](#template) |  |
| transform |  | [Template](#template) |  |
