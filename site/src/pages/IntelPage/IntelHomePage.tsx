import { useQuery } from "react-query";
import IntelLayout from "./IntelLayout";
import { intelReport } from "api/queries/intel";
import { useDashboard } from "modules/dashboard/useDashboard";
import { Card } from "@mui/material";
import { useTheme } from "@emotion/react";
import { FC, PropsWithChildren } from "react";

const IntelHomePage = () => {
  const theme = useTheme();
  const { organizationId } = useDashboard();
  const report = useQuery(intelReport(organizationId));

  if (report.isLoading) {
    return <div>Loading...</div>;
  }
  if (!report.data) {
    return <div>No data</div>;
  }

  return (
    <IntelLayout>
      <div
        css={{
          display: "grid",
          gridTemplateColumns: "0.7fr 0.3fr",
          gap: 16,
        }}
      >
        <Panel>
          <div>
            Popular Repos
          </div>
          <div>

          </div>
        </Panel>
        <Panel>
          <div
            css={{
              fontSize: 24,
            }}
          >
            Commands in the Last Week
          </div>
          <div
            css={{
              fontSize: 64,
              lineHeight: 1,
              margin: `${theme.spacing(1)} 0px`,
            }}
          >
            {report.data.invocations}
          </div>
        </Panel>
      </div>
    </IntelLayout>
  );
};

const Panel: FC<PropsWithChildren> = ({ children }) => {
  const theme = useTheme();

  return (
    <div
      css={{
        backgroundColor: theme.palette.background.paper,
        borderRadius: theme.shape.borderRadius,
        boxShadow: theme.shadows[1],
        padding: theme.spacing(3),
        marginBottom: theme.spacing(2),
      }}
    >
      {children}
    </div>
  );
};

export default IntelHomePage;
