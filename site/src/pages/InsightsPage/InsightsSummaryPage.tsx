import { useTheme } from "@emotion/react";
import type { FC, HTMLAttributes } from "react";
import InsightsChart from "./InsightsChart";

const InsightsSummaryPage = () => {
  return (
    <div>
      <Panel>
        <PanelHeader>
          <PanelTitle>
            Development Environment Consistency
          </PanelTitle>
        </PanelHeader>
        <PanelContent>
          Your Coder environments are becoming more consistent. Your local environments are becoming less consistent.
          <InsightsChart lines={[{
            label: "Coder",
            pointBackgroundColor: "#FFA726",
            pointBorderColor: "#FFA726",
            borderColor: "#FFA726",
            data: [{
              date: "2024-03-24",
              amount: 10,
            }, {
              date: "2024-03-25",
              amount: 20,
            }, {
              date: "2024-03-26",
              amount: 30,
            }]
          }, {
            label: "Local",
            pointBackgroundColor: "#26ff72",
            pointBorderColor: "#26ff5c",
            borderColor: "#6b26ff",
            data: [{
              date: "2024-03-24",
              amount: 30,
            }, {
              date: "2024-03-25",
              amount: 20,
            }, {
              date: "2024-03-26",
              amount: 10,
            }]
          }]}  interval="day" />
        </PanelContent>
      </Panel>

      <Panel>
        <PanelHeader>
          <PanelTitle>
            Time Between Commits
          </PanelTitle>
        </PanelHeader>
        <PanelContent>
          In the Coder environments, the time between commits is decreasing. In the local environments, the time between commits is increasing.
        </PanelContent>
      </Panel>

      <Panel>
        <PanelHeader>
          <PanelTitle>
            Longest Commands
          </PanelTitle>
        </PanelHeader>
        <PanelContent>
          Looks like `make lint` is taking a lot longer this week than it was last week.
        </PanelContent>
      </Panel>

      <Panel>
        <PanelHeader>
          <PanelTitle>
            Editor Actions
          </PanelTitle>
        </PanelHeader>
        <PanelContent>
          Something
        </PanelContent>
      </Panel>
    </div>
  );
}

interface PanelProps extends HTMLAttributes<HTMLDivElement> {}

const Panel: FC<PanelProps> = ({ children, ...attrs }) => {
  const theme = useTheme();

  return (
    <div
      css={{
        borderRadius: 8,
        border: `1px solid ${theme.palette.divider}`,
        backgroundColor: theme.palette.background.paper,
        display: "flex",
        flexDirection: "column",
      }}
      {...attrs}
    >
      {children}
    </div>
  );
};

const PanelHeader: FC<HTMLAttributes<HTMLDivElement>> = ({
  children,
  ...attrs
}) => {
  return (
    <div css={{ padding: "20px 24px 24px" }} {...attrs}>
      {children}
    </div>
  );
};

const PanelTitle: FC<HTMLAttributes<HTMLDivElement>> = ({
  children,
  ...attrs
}) => {
  return (
    <div css={{ fontSize: 14, fontWeight: 500 }} {...attrs}>
      {children}
    </div>
  );
};

const PanelContent: FC<HTMLAttributes<HTMLDivElement>> = ({
  children,
  ...attrs
}) => {
  return (
    <div css={{ padding: "0 24px 24px", flex: 1 }} {...attrs}>
      {children}
    </div>
  );
};

export default InsightsSummaryPage
