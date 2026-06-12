import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@vllnt/ui";

import { COMMANDS } from "@/lib/docs";

/**
 * Renders the full dig command surface as a reference table.
 *
 * @returns the command-reference table
 */
export function CommandTable(): React.JSX.Element {
  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Command</TableHead>
            <TableHead>What it does</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {COMMANDS.map((command) => (
            <TableRow key={command.usage}>
              <TableCell className="whitespace-nowrap align-top font-mono text-xs">
                {command.usage}
              </TableCell>
              <TableCell className="align-top">
                <span className="font-medium">{command.summary}</span>{" "}
                <span className="text-sm text-muted-foreground">
                  {command.detail}
                </span>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
