import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
    initKioskMode,
    isKioskMode,
    setKioskMode,
    subscribeToKioskMode,
    getPulseBaseUrl,
    getPulseHostname,
    isPulseHttps,
    getPulseWebSocketUrl
} from '../url';

describe('url utils', () => {
    const originalLocation = window.location;
    const originalSessionStorage = window.sessionStorage;

    beforeEach(() => {
        // Reset window.location
        delete (window as any).location;
        window.location = {
            ...originalLocation,
            origin: 'http://localhost:3000',
            protocol: 'http:',
            hostname: 'localhost',
            port: '3000',
            search: '',
            host: 'localhost:3000'
        } as any;

        // Reset sessionStorage mock
        const storage: Record<string, string> = {};
        (window as any).sessionStorage = {
            getItem: vi.fn((key: string) => storage[key] || null),
            setItem: vi.fn((key: string, value: string) => { storage[key] = value; }),
            removeItem: vi.fn((key: string) => { delete storage[key]; }),
            clear: vi.fn(() => { for (const key in storage) delete storage[key]; }),
        } as any;
    });

    afterEach(() => {
        (window as any).location = originalLocation;
        (window as any).sessionStorage = originalSessionStorage;
        vi.restoreAllMocks();
    });

    describe('kiosk mode', () => {
        it('initKioskMode sets kiosk mode from URL param 1', () => {
            window.location.search = '?kiosk=1';
            initKioskMode();
            expect(window.sessionStorage.setItem).toHaveBeenCalledWith('pulse_kiosk_mode', 'true');
        });

        it('initKioskMode sets kiosk mode from URL param true', () => {
            window.location.search = '?kiosk=true';
            initKioskMode();
            expect(window.sessionStorage.setItem).toHaveBeenCalledWith('pulse_kiosk_mode', 'true');
        });

        it('initKioskMode removes kiosk mode from URL param 0', () => {
            window.location.search = '?kiosk=0';
            initKioskMode();
            expect(window.sessionStorage.removeItem).toHaveBeenCalledWith('pulse_kiosk_mode');
        });

        it('isKioskMode returns true if stored in session', () => {
            vi.mocked(window.sessionStorage.getItem).mockReturnValue('true');
            expect(isKioskMode()).toBe(true);
        });

        it('isKioskMode returns true if present in URL', () => {
            vi.mocked(window.sessionStorage.getItem).mockReturnValue(null);
            window.location.search = '?kiosk=1';
            expect(isKioskMode()).toBe(true);
        });

        it('setKioskMode updates storage and notifies listeners', () => {
            const listener = vi.fn();
            const unsubscribe = subscribeToKioskMode(listener);

            setKioskMode(true);
            expect(window.sessionStorage.setItem).toHaveBeenCalledWith('pulse_kiosk_mode', 'true');
            expect(listener).toHaveBeenCalledWith(true);

            setKioskMode(false);
            expect(window.sessionStorage.setItem).toHaveBeenCalledWith('pulse_kiosk_mode', 'false');
            expect(listener).toHaveBeenCalledWith(false);

            unsubscribe();
        });
    });

    describe('Pulse URLs', () => {
        it('getPulseBaseUrl returns window.location.origin', () => {
            expect(getPulseBaseUrl()).toBe('http://localhost:3000');
        });

        it('getPulseHostname returns hostname', () => {
            expect(getPulseHostname()).toBe('localhost');
        });

        it('isPulseHttps returns true for https', () => {
            (window.location as any).protocol = 'https:';
            Object.defineProperty(window.location, 'origin', {
                value: 'https://localhost:3000',
                writable: true,
                configurable: true
            });
            expect(isPulseHttps()).toBe(true);
        });

        it('getPulseWebSocketUrl returns correct ws URL', () => {
            const wsUrl = getPulseWebSocketUrl('/ws');
            expect(wsUrl).toContain('ws://localhost:3000/ws');
        });

        it('getPulseWebSocketUrl appends auth token if present', () => {
            vi.mocked(window.sessionStorage.getItem).mockImplementation((key) => {
                if (key === 'pulse_auth') return JSON.stringify({ type: 'token', value: 'secret' });
                return null;
            });
            const wsUrl = getPulseWebSocketUrl('/ws');
            expect(wsUrl).toContain('token=secret');
        });
    });
});
