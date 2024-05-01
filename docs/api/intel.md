# Intel

## List intel machines

### Code samples

```shell
# Example request using curl
curl -X GET http://coder-server:8080/api/v2/insights/daus \
  -H 'Accept: application/json' \
  -H 'Coder-Session-Token: API_KEY'
```

`GET /insights/daus`

### Parameters

| Name                        | In    | Type    | Required | Description                                                |
| --------------------------- | ----- | ------- | -------- | ---------------------------------------------------------- |
| `limit`                     | query | integer | false    | Page limit                                                 |
| `offset`                    | query | integer | false    | Page offset                                                |
| `operating_system`          | query | string  | false    | Regex to match a machine operating system against          |
| `operating_system_platform` | query | string  | false    | Regex to match a machine operating system platform against |
| `operating_system_version`  | query | string  | false    | Regex to match a machine operating system version against  |
| `architecture`              | query | string  | false    | Regex to match a machine architecture against              |
| `instance_id`               | query | string  | false    | Regex to match a machine instance ID against               |

### Example responses

> 200 Response

```json
{
  "count": 0,
  "intel_machines": [
    {
      "architecture": "string",
      "cpu_cores": 0,
      "created_at": "2019-08-24T14:15:22Z",
      "hostname": "string",
      "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
      "instance_id": "string",
      "memory_mb_total": 0,
      "operating_system": "string",
      "operating_system_platform": "string",
      "operating_system_version": "string",
      "organization_id": "7c60d51f-b44e-4682-87d6-449835ea4de6",
      "updated_at": "2019-08-24T14:15:22Z",
      "user_id": "a169451c-8525-4352-b8ca-070dd449a1a5"
    }
  ]
}
```

### Responses

| Status | Meaning                                                 | Description | Schema                                                                     |
| ------ | ------------------------------------------------------- | ----------- | -------------------------------------------------------------------------- |
| 200    | [OK](https://tools.ietf.org/html/rfc7231#section-6.3.1) | OK          | [codersdk.IntelMachinesResponse](schemas.md#codersdkintelmachinesresponse) |

To perform this operation, you must be authenticated. [Learn more](authentication.md).
