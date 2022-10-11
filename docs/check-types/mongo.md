## <img src='https://raw.githubusercontent.com/flanksource/flanksource-ui/main/src/icons/mongodb.svg' style='height: 32px'/> Mongo

??? example
     ```yaml
     apiVersion: canaries.flanksource.com/v1
     kind: Canary
     metadata:
       name: mongo
     spec:
       interval: 30
       mongodb:
         - connection: mongodb://$(username):$(password)@mongo.default.svc:27017/?authSource=admin
           description: mongo ping
           auth:
             username:
               value: mongoadmin
             password:
               value: secret
       dns:
         - query: mongo.default.svc
     
     ```

| Field | Description | Scheme | Required |
| ----- | ----------- | ------ | -------- |
| auth |  | [Authentication](#authentication) |  |
| **connection** |  | string | Yes |
| description | Description for the check | string |  |
| icon | Icon for overwriting default icon on the dashboard | string |  |
| name | Name of the check | string |  |
