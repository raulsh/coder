import { css } from "@emotion/react";
import IntelEmpty from "./IntelEmpty";

const IntelSummaryPage = () => {
  return (
    <div
      css={css`
        display: grid;
        grid-template-columns: 1fr 1fr 1fr 1fr 1fr;
        gap: 16px;
        padding: 16px;
      `}
    >
      <IntelEmpty />
    </div>
  );
};

export default IntelSummaryPage;
