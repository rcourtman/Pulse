// Real shared table/grid actions with synthetic data, not an installed journey.
import { createSignal } from 'solid-js';
import { render } from 'solid-js/web';
import { Table, TableBody, TableCell, TableRow } from '../src/components/shared/Table';
import { SummaryRowActionButton } from '../src/components/shared/SummaryRowActionButton';
import { PulseDataGrid } from '../src/components/shared/PulseDataGrid';
import '../src/index.css';

function Fixture() {
  const [expanded, setExpanded] = createSignal(false);
  const [rowActions, setRowActions] = createSignal(0);
  const [childActions, setChildActions] = createSignal(0);
  const [enabled, setEnabled] = createSignal(true);
  const [gridActions, setGridActions] = createSignal(0);
  const toggle = () => {
    setRowActions((count) => count + 1);
    setExpanded((value) => !value);
  };
  const child = (event: MouseEvent) => {
    event.stopPropagation();
    setChildActions((count) => count + 1);
  };
  return (
    <main class="space-y-4 p-4">
      <h1>Shared table touch verification</h1>
      <button type="button" onClick={() => setEnabled((value) => !value)}>
        Toggle row action
      </button>
      <output aria-label="Action state" class="block break-all text-xs">
        {JSON.stringify({
          expanded: expanded(),
          rowActions: rowActions(),
          childActions: childActions(),
          enabled: enabled(),
          gridActions: gridActions(),
        })}
      </output>
      <Table>
        <TableBody>
          <TableRow data-testid="control-row" onClick={enabled() ? toggle : undefined}>
            <TableCell>
              <SummaryRowActionButton
                kind="disclosure"
                subjectLabel="shared resource"
                expanded={expanded()}
                hideWhenRowTappableOnMobile={false}
                onAction={toggle}
              />
              <span>Shared resource name</span>
            </TableCell>
            <TableCell>
              <button type="button" onClick={child}>
                Child action
              </button>
              <a href="#native-link" onClick={child}>
                Native link
              </a>
              <input
                type="checkbox"
                aria-label="Child checkbox"
                onClick={(event) => event.stopPropagation()}
              />
            </TableCell>
          </TableRow>
          <TableRow>
            <TableCell>Static row</TableCell>
            <TableCell>No action</TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <PulseDataGrid<{ id: string; name: string }>
        data={[{ id: 'grid-one', name: 'Grid resource' }]}
        keyExtractor={(row) => row.id}
        onRowClick={() => setGridActions((count) => count + 1)}
        columns={[
          { key: 'name', label: 'Name' },
          {
            key: 'action',
            label: 'Action',
            render: () => (
              <button type="button" onClick={child}>
                Grid child action
              </button>
            ),
          },
        ]}
      />
    </main>
  );
}
render(() => <Fixture />, document.getElementById('root')!);
