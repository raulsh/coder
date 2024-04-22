import { useState, useEffect, useRef } from "react";
import { watchBuildLogsByBuildId } from "api/api";
import type { ProvisionerJobLog } from "api/typesGenerated";
import { displayError } from "components/GlobalSnackbar/utils";

export const useWorkspaceBuildLogs = (
  // buildId is optional because sometimes the build is not loaded yet
  buildId: string | undefined,
  enabled: boolean = true,
) => {
  const [logs, setLogs] = useState<ProvisionerJobLog[]>();
  const socket = useRef<WebSocket>();

  useEffect(() => {
    if (!buildId || !enabled) {
      socket.current?.close();
      return;
    }

    // Every time this hook is called reset the values
    setLogs(undefined);

    socket.current = watchBuildLogsByBuildId({
      buildId,
      after: -1, // Retrieve all the logs
      onMessage: (log) => {
        setLogs((current) => {
          if (!current) {
            return [log];
          }

          return [...current, log];
        });
      },
      onError: () => {
        displayError("Error on getting the build logs");
      },
    });

    return () => {
      socket.current?.close();
    };
  }, [buildId, enabled]);

  return logs;
};
