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
  const { organizationId } = useDashboard();
  const cohortsQuery = useQuery(intelCohorts(organizationId));

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
  const { organizationId } = useDashboard();
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
