<!-- DO NOT EDIT | GENERATED CONTENT -->

# inteld start

Start the Intel Daemon

## Usage

```console
coder inteld start [flags]
```

## Options

### --verbose

|             |                                          |
| ----------- | ---------------------------------------- |
| Type        | <code>bool</code>                        |
| Environment | <code>$CODER_INTEL_DAEMON_VERBOSE</code> |
| Default     | <code>false</code>                       |

Output debug-level logs.

### --log-human

|             |                                                |
| ----------- | ---------------------------------------------- |
| Type        | <code>string</code>                            |
| Environment | <code>$CODER_INTEL_DAEMON_LOGGING_HUMAN</code> |
| Default     | <code>/dev/stderr</code>                       |

Output human-readable logs to a given file.

### --log-json

|             |                                               |
| ----------- | --------------------------------------------- |
| Type        | <code>string</code>                           |
| Environment | <code>$CODER_INTEL_DAEMON_LOGGING_JSON</code> |

Output JSON logs to a given file.

### --log-stackdriver

|             |                                                      |
| ----------- | ---------------------------------------------------- |
| Type        | <code>string</code>                                  |
| Environment | <code>$CODER_INTEL_DAEMON_LOGGING_STACKDRIVER</code> |

Output Stackdriver compatible logs to a given file.

### --log-filter

|             |                                             |
| ----------- | ------------------------------------------- |
| Type        | <code>string-array</code>                   |
| Environment | <code>$CODER_INTEL_DAEMON_LOG_FILTER</code> |

Filter debug logs by matching against a given regex. Use .\* to match all debug logs.

### --invoke-directory

|             |                                                   |
| ----------- | ------------------------------------------------- |
| Type        | <code>string</code>                               |
| Environment | <code>$CODER_INTEL_DAEMON_INVOKE_DIRECTORY</code> |
| Default     | <code>~/.coder-intel/bin</code>                   |

The directory where binaries are aliased to and overridden in the $PATH so they can be tracked.

### --instance-id

|             |                                               |
| ----------- | --------------------------------------------- |
| Type        | <code>string</code>                           |
| Environment | <code>$CODER_INTEL_DAEMON_INSTANCE_ID</code>  |
| Default     | <code>a1acbdbfe2274834a19f3682db50dc2c</code> |

The instance ID of the machine running the intel daemon. This is used to identify the machine.
