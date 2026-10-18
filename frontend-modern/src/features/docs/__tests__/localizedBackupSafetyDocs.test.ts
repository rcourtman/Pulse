import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { renderDocMarkdown } from '../docMarkdown';

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../..');
const repoRoot = path.resolve(frontendRoot, '..');
const readDoc = (name: string) => readFileSync(path.join(repoRoot, 'docs', name), 'utf8');
const render = (source: string, slug: string) => {
  const article = document.createElement('article');
  article.innerHTML = renderDocMarkdown(source, slug);
  return article;
};

const locales = [
  {
    locale: 'de',
    heading: 'proxmox-sicherheit-bei-backups',
    warning: 'API-only bedeutet nicht ohne Gastagent',
    readOnly: 'Nur Leserechte zu haben, beweist keine Backup-Sicherheit',
    defer: 'keine manuellen Gastagent-Abfragen senden',
    planned: 'vor einem geplanten Backup mit Dateisystem-Freeze',
    issued: 'bricht bereits gesendete Gastagent-Aufrufe nicht ab',
    outage: 'stehen Monitoring und Warnmeldungen nicht zur Verfügung',
    ok: 'OK beweist nicht, dass der Gast wieder aufgetaut ist',
    independent: 'nach dessen Ende, ohne Pulse oder QEMU Guest Agent',
    checks: [
      'Die Dateisysteme sind wieder aufgetaut.',
      'Frische, erfolgreiche Schreibvorgänge der Workloads auf jedem vom Backup erfassten Dateisystem.',
      'Die Workloads funktionieren.',
    ],
    failed: 'Fehlt eine Prüfung oder schlägt sie fehl, die Pause beibehalten',
    console: 'Eine Konsolenverbindung oder ein erfolgreicher Lesezugriff genügt nicht',
    restore: 'vor der Pause aktiv waren',
    unknown: 'unbekannte vorherige Zustände sind keine Erlaubnis zum Starten',
    lock: 'Keine Backup-Sperren löschen oder Freeze deaktivieren',
  },
  {
    locale: 'es',
    heading: 'proxmox-seguridad-durante-las-copias-de-seguridad',
    warning: 'API-only no significa sin agente de invitado',
    readOnly: 'permisos de solo lectura no demuestran que la copia sea segura',
    defer: 'ni envíes consultas manuales al agente de invitado',
    planned: 'antes de una copia programada con congelación',
    issued: 'no cancela las consultas ya enviadas al agente de invitado',
    outage: 'no hay monitoreo ni alertas de Pulse',
    ok: 'OK de la copia no demuestra que el invitado se haya descongelado',
    independent: 'posteriores a su finalización, sin Pulse ni QEMU Guest Agent',
    checks: [
      'Los sistemas de archivos se han descongelado.',
      'Escrituras recientes y correctas de las cargas de trabajo en cada sistema de archivos incluido en la copia.',
      'Las cargas de trabajo funcionan.',
    ],
    failed: 'Si falta una comprobación o falla, mantén la pausa',
    console: 'Una conexión a la consola o una lectura correcta no basta',
    restore: 'estaban activos antes de la pausa',
    unknown: 'un estado anterior desconocido no autoriza su inicio',
    lock: 'No borres bloqueos de copias ni desactives la congelación',
  },
];

for (const copy of locales) {
  const slug = `i18n/${copy.locale}/README`;
  const source = readDoc(`${slug}.md`);
  const safety = () => {
    const article = render(source, slug);
    const heading = article.querySelector(`#${copy.heading}`);
    expect(heading).not.toBeNull();
    const section = document.createElement('section');
    for (let node = heading!.nextElementSibling; node && !/^H[23]$/.test(node.tagName);) {
      const next = node.nextElementSibling;
      section.appendChild(node);
      node = next;
    }
    return section;
  };
  const text = () => safety().textContent!.replace(/\s+/g, ' ');

  describe(`${copy.locale} getting-started backup safety`, () => {
    it('warns beside API-only setup, without silently changing installation commands', () => {
      expect(text()).toContain(copy.warning);
      expect(text()).toContain(copy.readOnly);
      expect(text()).toContain(copy.defer);
      const section = safety();
      expect(section.querySelector('pre, code')).toBeNull();
      const article = render(source, slug);
      const heading = article.querySelector(`#${copy.heading}`)!;
      expect(heading.previousElementSibling?.textContent).toContain('API-only');
    });

    it('links the deployment-specific precaution, not an incident recovery command', () => {
      const link = safety().querySelector('a[href="/docs/VM_DISK_MONITORING#backup-safety"]');
      expect(link?.hasAttribute('data-doc-link')).toBe(true);
      expect(
        render(readDoc('VM_DISK_MONITORING.md'), 'VM_DISK_MONITORING').querySelector(
          '#backup-safety',
        ),
      ).not.toBeNull();
      expect(text()).toContain(copy.planned);
      expect(text()).toContain(copy.issued);
      expect(text()).toContain(copy.outage);
    });

    it('requires three independent post-backup checks including every covered filesystem', () => {
      expect(text()).toContain(copy.ok);
      expect(text()).toContain(copy.independent);
      const items = [...safety().querySelectorAll('ol > li')].map((item) =>
        item.textContent!.replace(/\s+/g, ' ').trim(),
      );
      expect(items).toEqual(copy.checks);
    });

    it('keeps a failed or unknown check paused and restores only prior-active controls', () => {
      expect(text()).toContain(copy.failed);
      expect(text()).toContain(copy.console);
      expect(text()).toContain(copy.restore);
      expect(text()).toContain(copy.unknown);
      expect(text()).toContain(copy.lock);
    });

    it('ships the same translated precaution in the public docs asset', () => {
      expect(readFileSync(path.join(frontendRoot, `public/docs/${slug}.md`), 'utf8')).toBe(source);
    });
  });
}
