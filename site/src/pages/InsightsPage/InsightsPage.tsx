import { Helmet } from "react-helmet-async";
import { pageTitle } from "utils/page";
import InsightsLayout from "./InsightsLayout";

const InsightsPage = () => {
  return (
    <>
      <Helmet>
        <title>{pageTitle("Insights")}</title>
      </Helmet>
      <InsightsLayout />
    </>
  );
};

export default InsightsPage;
