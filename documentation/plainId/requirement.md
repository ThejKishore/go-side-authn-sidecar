## Requirement

Create a plainId Authorization class that will be used in the Authorization middleware.
1)  The finegrain.go need to do the authorization call to the plainId service. (/api/runtime/5.0/decisions/permit-deny). This is configured in the authorization.yaml file.
example:
```yaml
finegrain-check:
  enabled: true
  validation-url: "http://localhost:8080/fga/api/runtime/5.0/decisions/permit-deny"
```
2) The finegrain.go need to do make sure it is constructing the Request object correctly using the authorization.yaml file and incoming request object.
The authorization.yaml file will also gives the details of how the body content should be constructed and also from where in the request object it should be extracting the values using json path.
The resource-map will have the multiple such entries. So the logic should be generic enough to handle multiple such entries.
The finegrain.go will need to identify the request definition is there in the authorization.yaml file and then construct the request object accordingly.


eg Authorization.yaml
```yaml
finegrain-check:
  resource-map:
    "[/mm/web/v1/transaction:POST]":
      roles: ["ROLE_USER"]
      ruleset-name: "mm-transaction"
      ruleset-id: "10201"
      body:
        transactionName: $.transactionName
        transactionAmount: $.transactionAmount
        tranTemplateUsed: $.tranTemplateUsed # if templateId is not present in the request body, then templateUsed will be false if the templateId is present in the request body then templateUsed will be true.
        fromAccountIds: $.fromAccount[*].accountId
        toAccountIds: $.toAccount[*].accountId
        fromAccountValues: $.fromAccount[*].accountValue
    "[/plt/web/v1/user/login:PUT]":
      roles: [ "ROLE_USER" ]
      ruleset-name: "plt-login"
      ruleset-id: "10201"
      body:
        username: $.username
        password: $.password
        type: $.type

    "[/plt/web/v1/user/*:PUT]":
      roles: [ "ROLE_ADMIN" ]
      ruleset-name: "plt-login"
      ruleset-id: "10201"
      body:
        username: $.username
        password: $.password
        type: $.type
```

eg Request Object & URL

Request URL: https://localhost:8080/mm/web/v1/transaction

Request Object:
```json
{
  "transactionName": "Test",
  "transactionAmount": 100,
  "tranTemplateID": "TestTemplate",
  "fromAccount": [
    {
      "accountId": "1234567890",
      "accountValue": 10
    },{
      "accountId": "1234567891",
      "accountValue": 80
    },{
      "accountId": "1234567892",
      "accountValue": 10
    }
  ],
  "toAccount": [
    {
      "accountId": "1234567893",
      "accountValue": 10
    },{
      "accountId": "1234567894",
      "accountValue": 80
    },{
      "accountId": "1234567895",
      "accountValue": 10
    }
  ],
  "recipientId": "1234567890",
  "recipientName": "John Doe",
  "recipientAddress": "123 Main St,",
  "recipientCity": "Anytown",
  "recipientState": "NY",
  "recipientZip": "12345",
  "recipientCountry": "USA"
}
```


e.g. PlainId Request Object
```json
{
  "method": "POST",
  "headers": {
    "x-request-id": "8CDAC3e6r4D252ABE60EFD7A31AFEEBA", // RequestId from the incoming request
    "Authorization": "Bearer eyJhbG...lXvZQ" , // Incoming Authorization header JWT token
  },
  "uri": {
    "schema": "https",
    "authority": {
      "param1": "val1",
      "param2": "val2"
    },
    "path": [
      "/mm/web/v1/transaction",
      "mm",
      "web",
      "v1",
      "transaction"
    ],
    "query": {
      "details": true,
      "type": 2
    }
  },
  "body": {
    "transactionName": "Test",
    "transactionAmount": "100",
    "tranTemplateUsed": "TestTemplate",
    "fromAccountIds": ["1234567890","1234567891","1234567892"],
    "toAccountIds": ["1234567893","1234567894","1234567895"],
    "fromAccountValues" : ["10","80","10"]
  },
  "meta": {
    "runtimeFineTune": {
      "combinedMultiValue": false
    }
  }
}
```
PlainId API Reference:
* [API-v5-Contract](https://docs.plainid.io/apidocs/v5-permit-deny)
* [API-V5-Documentation](https://docs.plainid.io/apidocs/v5-endpoint-for-api-access)
* [json_path](https://jsonpath.com/)