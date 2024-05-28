import { Margins } from "components/Margins/Margins";
import { PageHeaderActions } from "components/PageHeader/FullWidthPageHeader";
import { PageHeader, PageHeaderTitle } from "components/PageHeader/PageHeader";
import { type FC, type PropsWithChildren } from "react";
import { Helmet } from "react-helmet-async";
import { Outlet } from "react-router-dom";
import { pageTitle } from "utils/page";

const IntelLayout: FC<PropsWithChildren> = ({ children = <Outlet /> }) => {
  return (
    <Margins>
      <Helmet>
        <title>{pageTitle("Intel")}</title>
      </Helmet>
      <PageHeader>
        <PageHeaderTitle>Intel</PageHeaderTitle>
        <PageHeaderActions>
          Create Cohort
        </PageHeaderActions>
      </PageHeader>
      {children}
    </Margins>
  );
};

export default IntelLayout;
