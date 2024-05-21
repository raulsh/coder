# Intel

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
  "metadata_match": {
    "property1": {},
    "property2": {}
  },
  "name": "string",
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

## List intel machines

### Code samples

```shell
# Example request using curl
curl -X GET http://coder-server:8080/api/v2/organizations/{organization}/intel/machines \
  -H 'Accept: application/json' \
  -H 'Coder-Session-Token: API_KEY'
```

`GET /organizations/{organization}/intel/machines`

### Parameters

| Name           | In    | Type         | Required | Description                                     |
| -------------- | ----- | ------------ | -------- | ----------------------------------------------- |
| `organization` | path  | string(uuid) | true     | Organization ID                                 |
| `limit`        | query | integer      | false    | Page limit                                      |
| `offset`       | query | integer      | false    | Page offset                                     |
| `metadata`     | query | string       | false    | A JSON object to match machine metadata against |

### Example responses

> 200 Response

```json
{
  "count": 0,
  "intel_machines": [
    {
      "created_at": "2019-08-24T14:15:22Z",
      "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
      "instance_id": "string",
      "metadata": {
        "property1": "string",
        "property2": "string"
      },
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

## Get intel report

### Code samples

```shell
# Example request using curl
curl -X GET http://coder-server:8080/api/v2/organizations/{organization}/intel/report?cohort_id=497f6eca-6276-4993-bfeb-53cbbbba6f08 \
  -H 'Accept: application/json' \
  -H 'Coder-Session-Token: API_KEY'
```

`GET /organizations/{organization}/intel/report`

### Parameters

| Name           | In    | Type         | Required | Description     |
| -------------- | ----- | ------------ | -------- | --------------- |
| `organization` | path  | string(uuid) | true     | Organization ID |
| `cohort_id`    | query | string(uuid) | true     | Cohort ID       |
| `starts_at`    | query | string(date) | false    | Starts at       |

### Example responses

> 200 Response

```json
{
  "git_auth_providers": {
    "property1": "string",
    "property2": "string"
  },
  "intervals": [
    {
      "binary_args": ["string"],
      "binary_name": "string",
      "binary_paths": {
        "property1": 0,
        "property2": 0
      },
      "ends_at": "2019-08-24T14:15:22Z",
      "exit_codes": {
        "property1": 0,
        "property2": 0
      },
      "git_remote_urls": {
        "property1": 0,
        "property2": 0
      },
      "id": "497f6eca-6276-4993-bfeb-53cbbbba6f08",
      "machine_metadata": {
        "property1": {
          "property1": 0,
          "property2": 0
        },
        "property2": {
          "property1": 0,
          "property2": 0
        }
      },
      "median_duration_ms": 0,
      "starts_at": "2019-08-24T14:15:22Z",
      "total_invocations": 0,
      "unique_machines": 0,
      "working_directories": {
        "property1": 0,
        "property2": 0
      }
    }
  ],
  "invocations": 0
}
```

### Responses

| Status | Meaning                                                 | Description | Schema                                                 |
| ------ | ------------------------------------------------------- | ----------- | ------------------------------------------------------ |
| 200    | [OK](https://tools.ietf.org/html/rfc7231#section-6.3.1) | OK          | [codersdk.IntelReport](schemas.md#codersdkintelreport) |

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
