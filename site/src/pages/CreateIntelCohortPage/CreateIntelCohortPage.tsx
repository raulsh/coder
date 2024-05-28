import { css } from "@emotion/react";
import {
  Button,
  Chip,
  FormControl,
  TextField
} from "@mui/material";
import { createIntelCohort } from "api/queries/intel";
import * as TypesGen from "api/typesGenerated";
import { FormFields, FormSection, HorizontalForm } from "components/Form/Form";
import { FormFooter } from "components/FormFooter/FormFooter";
import { IconField } from "components/IconField/IconField";
import { Margins } from "components/Margins/Margins";
import {
  PageHeader,
  PageHeaderSubtitle,
  PageHeaderTitle,
} from "components/PageHeader/PageHeader";
import { FormikContextType, useFormik } from "formik";
import { useDashboard } from "modules/dashboard/useDashboard";
import { useCallback, useState } from "react";
import { useMutation } from "react-query";
import { useNavigate } from "react-router-dom";
import {
  getFormHelpers,
  nameValidator,
  onChangeTrimmed,
} from "utils/formUtils";
import * as Yup from "yup";

const CreateIntelCohortPage = () => {
  const { organizationId } = useDashboard()
  const navigate = useNavigate()
  const createIntelCohortMutation = useMutation(createIntelCohort(organizationId))
  const form: FormikContextType<TypesGen.CreateIntelCohortRequest> =
    useFormik<TypesGen.CreateIntelCohortRequest>({
      initialValues: {
        name: "",
        description: "",
        icon: "",
        tracked_executables: [],
        metadata_match: {},
      },
      validationSchema: Yup.object({
        name: nameValidator("Cohort Name"),
      }),
      enableReinitialize: true,
      onSubmit: async (request) => {
        await createIntelCohortMutation.mutateAsync(request)
        navigate("/intel")
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

        {/* <FormSection
          title="Machines"
          description="The Cohort will target all registered machines by default."
        >
          <FormFields>
            {form.values.metadata_match && Object.entries(form.values.metadata_match).map(([key, value]) => (
              <Stack key={key} direction="row">
                <TextField
                  {...getFieldHelpers(`metadata_match.${key}`)}
                  disabled={isSubmitting}
                  // resetMutation facilitates the clearing of validation errors
                  onChange={onChangeTrimmed(form)}
                  fullWidth
                  label={key}
                  helperText="e.g: 22.02"
                />
                <Button
                  onClick={() => {
                    const newMetadataMatch = { ...form.values.metadata_match };
                    delete newMetadataMatch[key];
                    form.setFieldValue("metadata_match", newMetadataMatch);
                  }}
                >
                  Remove
                </Button>
              </Stack>
            ))}
            <Button onClick={() => {
              form.setFieldValue("metadata_match", {
                ...form.values.metadata_match,
                "": "",
              });
            }}>
              New Metadata Match
            </Button>
          </FormFields>
        </FormSection> */}

        <FormSection title="Tracked Executables" description="Machines that match the selector above will track binaries specified here. On Windows, `.exe` is automatically appended.">
          <FormFields>
            <IntelBinariesInput
              value={form.values.tracked_executables}
              onChange={(value) => form.setFieldValue("tracked_executables", value)}
            />
          </FormFields>
        </FormSection>

        <FormFooter
          isLoading={isSubmitting}
          onCancel={() => undefined}
        />
      </HorizontalForm>
    </Margins>
  );
};

const IntelBinariesInput: React.FC<{
  value: readonly string[];
  onChange: (value: string[]) => void;
}> = ({ value, onChange }) => {
  const [currentValue, setCurrentValue] = useState("");
  const handleOnDelete = useCallback(
    (index: number) => {
      const newValues = [...value];
      newValues.splice(index, 1);
      onChange(newValues);
    },
    [value, onChange],
  );
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLDivElement>) => {
      if (e.key !== "Enter") {
        return
      }
      const index = value.indexOf(currentValue)
      setCurrentValue("");
      if (index !== -1) {
        // Already exists!
        return
      }
      onChange([...value, currentValue]);
    },
    [value, onChange, currentValue],
  );

  return (
    <div>
      <FormControl>
        <TextField
          label="Binary Name"
          placeholder="e.g. go, node, pnpm, yarn"
          onKeyDown={handleKeyDown}
          value={currentValue}
          fullWidth
          onChange={(e) => setCurrentValue(e.target.value)}
          helperText="Press Enter to add a new binary."
        />
        <div css={css`
          display: flex;
          flex-wrap: wrap;
          gap: 4px;
          margin-top: 8px;
        `}>
          {value.map((value, index) => (
            <Chip
              key={value}
              size="small"
              label={value}
              onDelete={() => handleOnDelete(index)}
            />
          ))}
        </div>
      </FormControl>
    </div>
  );
};

export default CreateIntelCohortPage;
