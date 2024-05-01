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

## Create intel cohort

### Code samples

```shell
# Example request using curl
curl -X POST http://coder-server:8080/api/v2/organizations/{organization}/intel/cohorts \
  -H 'Content-Type: application/json' \
  -H 'Accept: */*' \
  -H 'Coder-Session-Token: API_KEY'
```

`POST /organizations/{organization}/intel/cohorts`

> Body parameter

```json
{
  "description": "string",
  "icon": "string",
  "name": "string",
  "regex_filters": {
    "architecture": "string",
    "instance_id": "string",
    "operating_system": "string",
    "operating_system_platform": "string",
    "operating_system_version": "string"
  },
  "tracked_executables": ["string"]
}
```

### Parameters

| Name           | In   | Type                                                                             | Required | Description                 |
| -------------- | ---- | -------------------------------------------------------------------------------- | -------- | --------------------------- |
| `organization` | path | string(uuid)                                                                     | true     | Organization ID             |
| `body`         | body | [codersdk.CreateIntelCohortRequest](schemas.md#codersdkcreateintelcohortrequest) | true     | Create intel cohort request |

### Example responses

> 201 Response

### Responses

| Status | Meaning                                                      | Description | Schema                                                 |
| ------ | ------------------------------------------------------------ | ----------- | ------------------------------------------------------ |
| 201    | [Created](https://tools.ietf.org/html/rfc7231#section-6.3.2) | Created     | [codersdk.IntelCohort](schemas.md#codersdkintelcohort) |

To perform this operation, you must be authenticated. [Learn more](authentication.md).

## Serve intel daemon

### Code samples

```shell
# Example request using curl
curl -X GET http://coder-server:8080/api/v2/organizations/{organization}/intel/serve?instance_id=string \
  -H 'Coder-Session-Token: API_KEY'
```

`GET /organizations/{organization}/intel/serve`

### Parameters

| Name                       | In    | Type         | Required | Description              |
| -------------------------- | ----- | ------------ | -------- | ------------------------ |
| `organization`             | path  | string(uuid) | true     | Organization ID          |
| `instance_id`              | query | string       | true     | Instance ID              |
| `cpu_cores`                | query | integer      | false    | Number of CPU cores      |
| `memory_total_mb`          | query | integer      | false    | Total memory in MB       |
| `hostname`                 | query | string       | false    | Hostname                 |
| `operating_system`         | query | string       | false    | Operating system         |
| `operating_system_version` | query | string       | false    | Operating system version |
| `architecture`             | query | string       | false    | Architecture             |

### Responses

| Status | Meaning                                                                  | Description         | Schema |
| ------ | ------------------------------------------------------------------------ | ------------------- | ------ |
| 101    | [Switching Protocols](https://tools.ietf.org/html/rfc7231#section-6.2.2) | Switching Protocols |        |

To perform this operation, you must be authenticated. [Learn more](authentication.md).
