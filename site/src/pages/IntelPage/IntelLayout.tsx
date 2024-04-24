import type { FC, PropsWithChildren } from "react";
import { Helmet } from "react-helmet-async";
import { Outlet, useLocation } from "react-router-dom";
import { FilterSearchMenu, OptionItem } from "components/Filter/filter";
import { useFilterMenu } from "components/Filter/menu";
import { Margins } from "components/Margins/Margins";
import { PageHeader, PageHeaderTitle } from "components/PageHeader/PageHeader";
import { TabLink, Tabs, TabsList } from "components/Tabs/Tabs";
import { pageTitle } from "utils/page";

const IntelLayout: FC<PropsWithChildren> = ({ children = <Outlet /> }) => {
  const location = useLocation();
  const paths = location.pathname.split("/");
  const activeTab = paths[2] ?? "summary";

  const cohortFilter = useFilterMenu({
    onChange: () => undefined,
    value: "Create Cohort",
    id: "cohort",
    getOptions: async (_) => {
      return [];
    },
    getSelectedOption: async () => {
      return null;
    },
    enabled: true,
  });

  return (
    <Margins>
      <Helmet>
        <title>{pageTitle("Intel")}</title>
      </Helmet>
      <PageHeader
        actions={
          <>
            <FilterSearchMenu menu={cohortFilter} label={cohortFilter.selectedOption ? cohortFilter.selectedOption : "Create Cohorts"} id="Cohort">
              {(itemProps) => (
                <OptionItem
                  option={itemProps.option}
                  isSelected={itemProps.isSelected}
                  left={<>{"Something"}</>}
                />
              )}
            </FilterSearchMenu>
          </>
        }
      >
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
