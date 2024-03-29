import type { FC, PropsWithChildren } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Margins } from "components/Margins/Margins";
import { PageHeader, PageHeaderTitle } from "components/PageHeader/PageHeader";
import { TabLink, Tabs, TabsList } from "components/Tabs/Tabs";
import { Helmet } from "react-helmet-async";
import { pageTitle } from "utils/page";

const InsightsLayout: FC<PropsWithChildren> = ({ children = <Outlet /> }) => {
  const location = useLocation();
  const paths = location.pathname.split("/");
  const activeTab = paths[2] ?? "summary";

  return (
    <Margins>
      <Helmet>
        <title>{pageTitle("Intel")}</title>
      </Helmet>
      <PageHeader>
        <PageHeaderTitle>Intel</PageHeaderTitle>
      </PageHeader>
      <Tabs active={activeTab}>
        <TabsList>
          <TabLink to="/intel" value="summary">
            Summary
          </TabLink>
          <TabLink to="/intel/tools" value="tools">
            Consistency
          </TabLink>
          <TabLink to="/intel/commands" value="commands">
            Commands
          </TabLink>
          <TabLink to="/intel/editors" value="editors">
            Editors
          </TabLink>
        </TabsList>
      </Tabs>
        {children}
    </Margins>
  );
};

export default InsightsLayout;
