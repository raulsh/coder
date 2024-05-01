import { useEffect, useMemo, useRef, useState, type FC, type PropsWithChildren } from "react";
import { Helmet } from "react-helmet-async";
import { Outlet, useLocation } from "react-router-dom";
import { FilterSearchMenu, OptionItem } from "components/Filter/filter";
import { useFilterMenu } from "components/Filter/menu";
import { Margins } from "components/Margins/Margins";
import { PageHeader, PageHeaderTitle } from "components/PageHeader/PageHeader";
import { TabLink, Tabs, TabsList } from "components/Tabs/Tabs";
import { pageTitle } from "utils/page";
import { useQuery } from "react-query";
import { intelCohorts } from "api/queries/intel";
import { useAuthenticated } from "contexts/auth/RequireAuth";
import { Button, Menu, MenuItem, MenuList } from "@mui/material";
import { KeyboardArrowDown } from "@mui/icons-material";
import { css } from "@emotion/react";
import { IntelCohort } from "api/typesGenerated";

const IntelLayout: FC<PropsWithChildren> = ({ children = <Outlet /> }) => {
  const location = useLocation();
  const paths = location.pathname.split("/");
  const activeTab = paths[2] ?? "summary";
  const { organizationId } = useAuthenticated();
  const cohortsQuery = useQuery(intelCohorts(organizationId));
  const cohortFilter = useFilterMenu({
    onChange: () => undefined,
    value: "All Cohorts",
    id: "cohort",
    getOptions: async (_) => {
      return (
        cohortsQuery.data?.map((cohort) => ({
          label: cohort.name,
          value: cohort.id,
        })) ?? []
      );
    },
    getSelectedOption: async () => {
      return null;
    },
    enabled: true,
  });

  if (cohortsQuery.isLoading) {
    return <div>Loading...</div>;
  }

  return (
    <Margins>
      <Helmet>
        <title>{pageTitle("Intel")}</title>
      </Helmet>
      <PageHeader
        actions={
          <>
            <CohortSelector />
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

const CohortSelector = () => {
  const { organizationId } = useAuthenticated();
  const cohortsQuery = useQuery(intelCohorts(organizationId))
  const cohortByID = useMemo(() => {
    if (!cohortsQuery.data) {
      return
    }
    const cohortByID: Record<string, IntelCohort> = {}
    cohortsQuery.data.forEach(cohort => {
      cohortByID[cohort.id] = cohort
    })
    return cohortByID
  }, [cohortsQuery.data])
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [selectedCohorts, setSelectedCohorts] = useState<string[]>([]);
  useEffect(() => {
    if (cohortByID) {
      setSelectedCohorts(Object.keys(cohortByID))
    }
  }, [cohortByID])

  if (cohortsQuery.isLoading) {
    return null
  }
  return (
    <>
      <Button
        ref={buttonRef}
        endIcon={<KeyboardArrowDown />}
        css={css`
          border-radius: 6px;
          justify-content: space-between;
          line-height: 120%;
        `}
        onClick={() => setIsMenuOpen(true)}
      >
        Select a Cohort
      </Button>

      <Menu
        open={isMenuOpen}
        onClose={() => setIsMenuOpen(false)}
        anchorEl={buttonRef.current}
      >
        <MenuList>
          {selectedCohorts.map(cohortID => {
            const cohort = cohortByID?.[cohortID]
            return (
              <MenuItem key={cohortID} onClick={() => {
                setSelectedCohorts(selectedCohorts.filter(id => id !== cohortID))
              }}>
                {cohort?.name}
              </MenuItem>
            )
          })}
        </MenuList>
      </Menu>
    </>
  );
};

export default IntelLayout;
