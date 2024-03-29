import { css, useTheme } from "@emotion/react";
import type { FC, HTMLAttributes } from "react";
import InsightsChart, { type InsightsChartProps } from "./InsightsChart";

const InsightsSummaryPage = () => {
  return (
    <div css={css`
      display: grid;
      grid-template-columns: 1fr 1fr 1fr 1fr 1fr;
      gap: 16px;
      padding: 16px;
    `}>
      <Panel css={css`
        grid-column: span 3;
      `}>
        <PanelHeader>
          <PanelTitle
            css={css`
              font-size: 20px;

              span {
                font-size: 14px;
                font-weight: 400;
                color: #666;
                margin-left: 16px;
                font-style: italic;
              }
            `}
          >
            Development Environment Consistency
            <span>
              Higher is Better
            </span>
          </PanelTitle>
        </PanelHeader>
        <PanelContent>
          <p css={css`
          margin-top: 0px;
          margin-bottom: 16px;`}>
          Your developers are using <b>97%</b> of the same toolchain in Coder workspaces.
          </p>
          <InsightsChart css={css`
            max-height: 400px;
            height: 100%;
          `} lines={fakeConsistencyData} interval="day" />
        </PanelContent>
      </Panel>

      <Panel css={css`
        grid-column: span 2;
      `}>
        <PanelHeader>
        <PanelTitle
            css={css`
              font-size: 20px;

              span {
                font-size: 14px;
                font-weight: 400;
                color: #666;
                margin-left: 16px;
                font-style: italic;
              }
            `}
          >
            Time Between Commits
            <span>
              Lower is Better
            </span>
          </PanelTitle>
        </PanelHeader>
        <PanelContent>

          <p css={css`
          margin-top: 0px;
          margin-bottom: 16px;`}>
          In Coder environments, your engineers commit 40% more.
          </p>
          <InsightsChart css={css`
            max-height: 400px;
            height: 100%;
          `} lines={fakeTimeCommitsData} interval="day" />
        </PanelContent>
      </Panel>

      <Panel>
        <PanelHeader>
          <PanelTitle>Longest Commands</PanelTitle>
        </PanelHeader>
        <PanelContent>
          Looks like `make lint` is taking a lot longer this week than it was
          last week.
        </PanelContent>
      </Panel>

      <Panel>
        <PanelHeader>
          <PanelTitle>Editor Actions</PanelTitle>
        </PanelHeader>
        <PanelContent>Something</PanelContent>
      </Panel>
    </div>
  );
};

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

const fakeTimeCommitsData: InsightsChartProps["lines"] = [
  {
    label: "Coder",
    data: [
      // 10
      12, 16, 17.7, 14, 15, 11, 13, 10, 11, 9,
      // 10
      9, 6, 7, 12, 11, 14, 12, 10, 8, 7,
      // 10
      9, 9.6, 7.7, 9, 5, 4, 3, 4, 6, 7,
    ].map((n, i) => ({
      date: `2024-03-${i}`,
      amount: n / 100,
    })),
    borderColor: "#60ff26",
    pointBackgroundColor: "#60ff26",
    pointBorderColor: "#60ff26",
  },
  {
    label: "Local Machines",
    pointBackgroundColor: "#ff5050",
    pointBorderColor: "#ff5050",
    borderColor: "#ff5050",
    data: [
      // 10
      20, 25, 19, 16, 19, 22, 26, 24, 22, 19,
      // 10
      20, 25, 19, 8, 19, 22, 26, 24, 22, 19,
      // 10
      20, 25, 19, 16, 19, 36, 26, 24, 22, 19,
    ].reverse().map((n, i) => ({
      date: `2024-03-${i}`,
      amount: n / 100,
    })),
  },
  {
    label: "VDI",
    pointBackgroundColor: "#5082ff",
    pointBorderColor: "#5082ff",
    borderColor: "#5082ff",
    data: [
      // 10
      36, 42, 39, 26, 29, 33, 34, 34, 28, 29,
      // 10
      40, 35, 36, 38, 39, 33, 30, 26, 28, 34,
      // 10
      36, 30, 29, 26, 29, 36, 46, 54, 42, 39,
    ].reverse().map((n, i) => ({
      date: `2024-03-${i}`,
      amount: (n * 1.4) / 100,
    })),
  },
];

const fakeConsistencyData: InsightsChartProps["lines"] = [
  {
    label: "Coder",
    data: [
      // 10
      95, 96, 97.7, 99, 95, 94, 93, 94, 96, 97,
      // 10
      95, 96, 97.7, 99, 95, 94, 93, 94, 96, 97,
      // 10
      95, 96, 97.7, 99, 95, 94, 93, 94, 96, 97,
    ].map((n, i) => ({
      date: `2024-03-${i}`,
      amount: n / 100,
    })),
    borderColor: "#60ff26",
    pointBackgroundColor: "#60ff26",
    pointBorderColor: "#60ff26",
  },
  {
    label: "Local Machines",
    pointBackgroundColor: "#ff5050",
    pointBorderColor: "#ff5050",
    borderColor: "#ff5050",
    data: [
      // 10
      20, 25, 19, 16, 19, 22, 26, 24, 22, 19,
      // 10
      20, 25, 19, 8, 19, 22, 26, 24, 22, 19,
      // 10
      20, 25, 19, 16, 19, 36, 26, 24, 22, 19,
    ].map((n, i) => ({
      date: `2024-03-${i}`,
      amount: n / 100,
    })),
  },
  {
    label: "VDI",
    pointBackgroundColor: "#5082ff",
    pointBorderColor: "#5082ff",
    borderColor: "#5082ff",
    data: [
      // 10
      36, 42, 39, 26, 29, 33, 34, 34, 28, 29,
      // 10
      40, 35, 36, 38, 39, 33, 30, 26, 28, 34,
      // 10
      36, 30, 29, 26, 29, 36, 46, 54, 42, 39,
    ].map((n, i) => ({
      date: `2024-03-${i}`,
      amount: n / 100,
    })),
  },
];

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

export default InsightsSummaryPage;
