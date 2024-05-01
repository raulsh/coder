import { css } from "@emotion/react";
import { Button, FormHelperText, TextField } from "@mui/material";
import { Margins } from "components/Margins/Margins";
import {
  PageHeader,
  PageHeaderSubtitle,
  PageHeaderTitle,
} from "components/PageHeader/PageHeader";
import { Stack } from "components/Stack/Stack";
import { FormikContextType, useFormik } from "formik";
import * as TypesGen from "api/typesGenerated";
import {
  getFormHelpers,
  nameValidator,
  onChangeTrimmed,
} from "utils/formUtils";
import { FormFields, FormSection, HorizontalForm } from "components/Form/Form";
import * as Yup from "yup";
import { IconField } from "components/IconField/IconField";

const CreateIntelCohortPage = () => {
  const form: FormikContextType<TypesGen.CreateIntelCohortRequest> =
    useFormik<TypesGen.CreateIntelCohortRequest>({
      initialValues: {
        name: "",
        description: "",
        icon: "",
        tracked_executables: [],
        regex_filters: {
          architecture: ".*",
          instance_id: ".*",
          operating_system: ".*",
          operating_system_platform: ".*",
          operating_system_version: ".*",
        },
      },
      validationSchema: Yup.object({
        name: nameValidator("Cohort Name"),
      }),
      enableReinitialize: true,
      onSubmit: (request) => {
        console.log("submit", request);
      },
    });
  const isSubmitting = false;
  const getFieldHelpers = getFormHelpers(form);

  return (
    <Margins size="medium">
      <PageHeader actions={<Button>Cancel</Button>}>
        <PageHeaderTitle>Create an Intel Cohort</PageHeaderTitle>

        <PageHeaderSubtitle condensed>
          <div
            css={css`
              max-width: 700px;
            `}
          >
            Define filters to monitor command invocations, detect redundant
            tools, identify time-consuming processes, and check for version
            inconsistencies in development environments.
          </div>
        </PageHeaderSubtitle>
      </PageHeader>

      <HorizontalForm onSubmit={form.handleSubmit}>
        <FormSection
          title="Display"
          description="The Cohort will be visible to everyone. Provide lots of details on which machines it should target!"
        >
          <FormFields>
            <TextField
              {...getFieldHelpers("name")}
              disabled={isSubmitting}
              // resetMutation facilitates the clearing of validation errors
              onChange={onChangeTrimmed(form)}
              fullWidth
              label="Name"
            />

            <TextField
              {...getFieldHelpers("description", {
                maxLength: 128,
              })}
              disabled={isSubmitting}
              rows={1}
              fullWidth
              label="Description"
            />

            <IconField
              {...getFieldHelpers("icon")}
              disabled={isSubmitting}
              onChange={onChangeTrimmed(form)}
              fullWidth
              onPickEmoji={(value) => form.setFieldValue("icon", value)}
            />
          </FormFields>
        </FormSection>

        <FormSection
          title="Machines"
          description="The Cohort will target all registered machines by default."
        >
          <FormFields>
            <Stack direction="row">
              <TextField
                {...getFieldHelpers("regex_filters.operating_system")}
                disabled={isSubmitting}
                // resetMutation facilitates the clearing of validation errors
                onChange={onChangeTrimmed(form)}
                fullWidth
                label="Operating System"
                helperText="e.g: linux, darwin, windows"
              />

              <TextField
                {...getFieldHelpers("regex_filters.operating_system_platform")}
                disabled={isSubmitting}
                // resetMutation facilitates the clearing of validation errors
                onChange={onChangeTrimmed(form)}
                fullWidth
                label="Operating System Platform"
                helperText="e.g: 22.02"
              />
            </Stack>

            <Stack direction="row">
              <TextField
                {...getFieldHelpers("regex_filters.operating_system_version")}
                disabled={isSubmitting}
                // resetMutation facilitates the clearing of validation errors
                onChange={onChangeTrimmed(form)}
                fullWidth
                label="Operating System Version"
                helperText="e.g: 22.02"
              />

              <TextField
                {...getFieldHelpers("regex_filters.architecture")}
                disabled={isSubmitting}
                // resetMutation facilitates the clearing of validation errors
                onChange={onChangeTrimmed(form)}
                fullWidth
                label="Architecture"
                helperText="e.g: arm, arm64, 386, amd64"
              />
            </Stack>
          </FormFields>
        </FormSection>
      </HorizontalForm>
    </Margins>
  );
};

export default CreateIntelCohortPage;
