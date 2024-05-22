import { css } from "@emotion/react";
import { KeyboardArrowDown } from "@mui/icons-material";
import { Button, Menu, MenuItem, MenuList } from "@mui/material";
import { intelCohorts } from "api/queries/intel";
import { IntelCohort } from "api/typesGenerated";
import { Margins } from "components/Margins/Margins";
import { PageHeader, PageHeaderTitle } from "components/PageHeader/PageHeader";
import { TabLink, Tabs, TabsList } from "components/Tabs/Tabs";
import { useDashboard } from "modules/dashboard/useDashboard";
import { useEffect, useMemo, useRef, useState, type FC, type PropsWithChildren } from "react";
import { Helmet } from "react-helmet-async";
import { useQuery } from "react-query";
import { Outlet, useLocation } from "react-router-dom";
import { pageTitle } from "utils/page";

const IntelLayout: FC<PropsWithChildren> = ({ children = <Outlet /> }) => {
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

export default IntelLayout;
