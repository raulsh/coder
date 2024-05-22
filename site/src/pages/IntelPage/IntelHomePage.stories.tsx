import type { Meta, StoryObj } from "@storybook/react";
import { getAuthorizationKey } from "api/queries/authCheck";
import { intelCohortsKey, intelReportKey } from "api/queries/intel";
import type { IntelCohort, IntelReport } from "api/typesGenerated";
import { AuthProvider } from "contexts/auth/AuthProvider";
import { RequireAuth } from "contexts/auth/RequireAuth";
import { permissionsToCheck } from "contexts/auth/permissions";
import {
  reactRouterOutlet,
  reactRouterParameters,
} from "storybook-addon-remix-react-router";
import {
  MockAppearanceConfig,
  MockAuthMethodsAll,
  MockBuildInfo,
  MockEntitlements,
  MockExperiments,
  MockUser,
} from "testHelpers/entities";
import IntelHomePage from "./IntelHomePage";

const meta: Meta<typeof IntelHomePage> = {
  title: "pages/IntelHomePage",
  component: RequireAuth,
  parameters: {
    queries: [
      { key: ["me"], data: MockUser },
      { key: ["authMethods"], data: MockAuthMethodsAll },
      { key: ["hasFirstUser"], data: true },
      { key: ["buildInfo"], data: MockBuildInfo },
      { key: ["entitlements"], data: MockEntitlements },
      { key: ["experiments"], data: MockExperiments },
      { key: ["appearance"], data: MockAppearanceConfig },
      {
        key: getAuthorizationKey({ checks: permissionsToCheck }),
        data: {},
      },
      {
        key: intelCohortsKey,
        data: [] as IntelCohort[],
      },
      {
        key: intelReportKey(),
        data: {
          invocations: 1000,
          git_auth_providers: {},
          intervals: [
            {
              binary_name: "go",
              binary_args: ["test"],
              binary_paths: {
                "/usr/bin/go": 100,
              },
              starts_at: "2021-01-01T00:00:00Z",
              ends_at: "2021-01-01T00:15:00Z",
              exit_codes: {},
              git_remote_urls: {},
              id: "",
              machine_metadata: {},
              median_duration_ms: 1,
              total_invocations: 100,
              unique_machines: 2,
              working_directories: {},
            },
          ],
        } as IntelReport,
      }
    ],
    reactRouter: reactRouterParameters({
      routing: reactRouterOutlet(
        {
          path: "/intel",
        },
        <IntelHomePage />,
      ),
    }),
  },
  decorators: [
    (Story) => (
      <AuthProvider>
        <Story />
      </AuthProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof IntelHomePage>;

export const Default: Story = {};
