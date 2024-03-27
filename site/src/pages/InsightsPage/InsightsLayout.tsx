import type { FC, PropsWithChildren } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Margins } from "components/Margins/Margins";
import { PageHeader, PageHeaderTitle } from "components/PageHeader/PageHeader";
import { TabLink, Tabs, TabsList } from "components/Tabs/Tabs";

const InsightsLayout: FC<PropsWithChildren> = ({ children = <Outlet /> }) => {
  const location = useLocation();
  const paths = location.pathname.split("/");
  const activeTab = paths[2] ?? "summary";

  return (
    <Margins>
      <PageHeader>
        <PageHeaderTitle>Insights</PageHeaderTitle>
      </PageHeader>
      <Tabs active={activeTab}>
        <TabsList>
          <TabLink to="/insights" value="summary">
            Summary
          </TabLink>
          <TabLink to="/insights/tools" value="tools">
            Tools
          </TabLink>
          <TabLink to="/insights/commands" value="commands">
            Commands
          </TabLink>
          <TabLink to="/insights/editors" value="editors">
            Editors
          </TabLink>
        </TabsList>
      </Tabs>
      <Margins>
        {children}
      </Margins>
    </Margins>
  );
};

export default InsightsLayout;
