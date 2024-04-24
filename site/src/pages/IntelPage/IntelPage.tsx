import { Helmet } from "react-helmet-async";
import { pageTitle } from "utils/page";
import IntelLayout from "./IntelLayout";

const IntelPage = () => {
  return (
    <>
      <Helmet>
        <title>{pageTitle("Insights")}</title>
      </Helmet>
      <IntelLayout />
    </>
  );
};

export default IntelPage;
