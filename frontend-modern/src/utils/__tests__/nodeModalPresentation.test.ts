import { describe, expect, it } from 'vitest';
import {
  buildNodeModalMonitoringPayload,
  getNodeEndpointHelp,
  getNodeEndpointPlaceholder,
  getNodeGuestUrlPlaceholder,
  getNodeModalDefaultFormData,
  getNodeModalTestResultPresentation,
  getNodeMonitoringCoverageCopy,
  getNodeProductName,
  getNodeTokenIdPlaceholder,
  getNodeUsernameHelp,
  getNodeUsernamePlaceholder,
} from '@/utils/nodeModalPresentation';

describe('nodeModalPresentation', () => {
  it('returns canonical default form data per proxmox node type', () => {
    expect(getNodeModalDefaultFormData('pve')).toMatchObject({
      authType: 'token',
      setupMode: 'agent',
      verifySSL: true,
      monitorPhysicalDisks: false,
    });
    expect(getNodeModalDefaultFormData('pbs')).toMatchObject({
      authType: 'token',
      setupMode: 'agent',
      verifySSL: true,
    });
    expect(getNodeModalDefaultFormData('pmg')).toMatchObject({
      authType: 'password',
      setupMode: 'agent',
      monitorMailStats: true,
      monitorQueues: true,
    });
  });

  it('returns canonical node product and field copy', () => {
    expect(getNodeProductName('pve')).toBe('Proxmox VE');
    expect(getNodeProductName('pbs')).toBe('Proxmox Backup Server');
    expect(getNodeProductName('pmg')).toBe('Proxmox Mail Gateway');

    expect(getNodeEndpointPlaceholder('pve')).toBe('https://proxmox.example.com:8006');
    expect(getNodeEndpointPlaceholder('pbs')).toBe('https://backup.example.com:8007');
    expect(getNodeEndpointPlaceholder('pmg')).toBe('https://mail-gateway.example.com:8006');

    expect(getNodeGuestUrlPlaceholder('pve')).toBe('https://pve.yourdomain.com');
    expect(getNodeGuestUrlPlaceholder('pbs')).toBe('https://pbs.yourdomain.com');
    expect(getNodeGuestUrlPlaceholder('pmg')).toBe('https://pmg.yourdomain.com');
  });

  it('returns canonical auth hints and monitoring coverage text', () => {
    expect(getNodeEndpointHelp('pve')).toBeNull();
    expect(getNodeEndpointHelp('pbs')).toContain('Default port is 8007');
    expect(getNodeEndpointHelp('pmg')).toContain('Default port is 8006');

    expect(getNodeUsernamePlaceholder('pve')).toBe('root@pam');
    expect(getNodeUsernamePlaceholder('pbs')).toBe('admin@pbs');
    expect(getNodeUsernamePlaceholder('pmg')).toBe('root@pam');

    expect(getNodeUsernameHelp('pve')).toBeNull();
    expect(getNodeUsernameHelp('pbs')).toContain('admin@pbs');
    expect(getNodeUsernameHelp('pmg')).toContain('api@pmg');

    expect(getNodeTokenIdPlaceholder('pve')).toBe('pulse-monitor@pve!pulse-token');
    expect(getNodeTokenIdPlaceholder('pbs')).toBe('pulse-monitor@pbs!pulse-token');

    expect(getNodeMonitoringCoverageCopy('pmg')).toContain('mail flow analytics');
    expect(getNodeMonitoringCoverageCopy('pve')).toContain('virtual machines');
  });

  it('builds canonical monitoring payloads per proxmox node type', () => {
    const pvePayload = buildNodeModalMonitoringPayload('pve', getNodeModalDefaultFormData('pve'));
    expect(pvePayload).toMatchObject({
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
      monitorBackups: true,
      monitorPhysicalDisks: false,
      physicalDiskPollingMinutes: 5,
    });

    const pbsPayload = buildNodeModalMonitoringPayload('pbs', getNodeModalDefaultFormData('pbs'));
    expect(pbsPayload).toMatchObject({
      monitorDatastores: true,
      monitorSyncJobs: true,
      monitorVerifyJobs: true,
      monitorPruneJobs: true,
      monitorGarbageJobs: true,
    });

    const pmgPayload = buildNodeModalMonitoringPayload('pmg', getNodeModalDefaultFormData('pmg'));
    expect(pmgPayload).toMatchObject({
      monitorMailStats: true,
      monitorQueues: true,
      monitorQuarantine: true,
      monitorDomainStats: false,
    });
  });

  it('returns canonical node test result presentation', () => {
    expect(getNodeModalTestResultPresentation('success')).toMatchObject({
      icon: 'success',
      panelClass: expect.stringContaining('bg-green-50'),
    });
    expect(getNodeModalTestResultPresentation('warning')).toMatchObject({
      icon: 'warning',
      panelClass: expect.stringContaining('bg-amber-50'),
    });
    expect(getNodeModalTestResultPresentation('error')).toMatchObject({
      icon: 'error',
      panelClass: expect.stringContaining('bg-red-50'),
    });
  });
});
