import Button from "@mui/material/Button"
import { TableEmpty } from "components/TableEmpty/TableEmpty"

const IntelEmpty = () => {
  return (
    <TableEmpty
      message="Install the Daemon"
      description="Start collecting data by installing the Coder daemon in your development environment."
      cta={
        <Button>
          Install Daemon
        </Button>
      }
    />
  )
}

export default IntelEmpty
