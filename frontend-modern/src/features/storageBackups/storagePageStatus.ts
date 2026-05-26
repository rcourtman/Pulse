export const isStoragePoolLoading = (
  loading: boolean,
  view: 'pools' | 'disks',
  filteredRecordCount: number,
): boolean => loading && view === 'pools' && filteredRecordCount === 0;
