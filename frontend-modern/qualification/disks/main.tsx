import { render } from 'solid-js/web';
import { DisksCard } from '../../src/components/shared/cards/DisksCard';
import '../../src/index.css';
const params = new URLSearchParams(location.search);
document.documentElement.classList.toggle('dark', params.get('theme') === 'dark');
const disks = Array.from({ length: Number(params.get('count') ?? 24) }, (_, i) => ({
  mountpoint: `/mnt/disk-${i}`,
  total: 1000000000,
  used: 500000000,
  free: 500000000,
  usage: 0.5,
}));
render(
  () => (
    <main class="bg-surface text-base-content p-4" style={{ width: '440px' }}>
      <button>Before disks</button>
      <DisksCard disks={disks} />
      <button>After disks</button>
    </main>
  ),
  document.getElementById('root')!,
);
