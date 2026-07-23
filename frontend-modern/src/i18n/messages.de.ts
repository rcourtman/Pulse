import type { I18nCatalog } from './messages';

export const DE_MESSAGE_OVERRIDES = {
  'pricing.handoff.description.afterLink': '.',
  'pricing.handoff.description.beforeLink': 'Wenn die Weiterleitung nicht automatisch startet,',
  'pricing.handoff.link.publicPricing': 'weiter zur oeffentlichen Preisseite',
  'pricing.handoff.link.pulseAccount': 'weiter zu Pulse Account',
  'pricing.handoff.title.publicPricing': 'Weiterleitung zu den Preisen',
  'pricing.handoff.title.pulseAccount': 'Weiterleitung zu Pulse Account',
  'alerts.activation.label.enabled': 'Benachrichtigungen aktiviert',
  'alerts.activation.label.disabled': 'Benachrichtigungen pausiert',
  'alerts.activation.toast.activated':
    'Benachrichtigungen aktiviert. Sie erhalten jetzt Hinweise, wenn Probleme erkannt werden.',
  'alerts.activation.toast.activateFailed':
    'Benachrichtigungen konnten nicht aktiviert werden. Bitte versuchen Sie es erneut.',
  'alerts.activation.toast.deactivated':
    'Benachrichtigungen pausiert. Pulse erkennt und zeigt aktive Warnmeldungen weiterhin an.',
  'alerts.activation.toast.deactivateFailed':
    'Benachrichtigungen konnten nicht pausiert werden. Bitte versuchen Sie es erneut.',
  'alerts.assistant.action.investigate': 'Warnmeldung {alertIdentifier} untersuchen',
  'alerts.assistant.button.full': 'Pulse Assistant zu dieser Warnmeldung fragen',
  'alerts.assistant.button.text': 'Pulse Assistant fragen',
  'alerts.assistant.detail.currentMetric':
    'Aktueller Wert {currentValue}; Schwellwert {thresholdValue}',
  'alerts.assistant.detail.message': 'Meldung: {message}',
  'alerts.assistant.detail.node': 'Knoten: {node}',
  'alerts.assistant.duration.hoursMinutes': '{hours}h {minutes}m',
  'alerts.assistant.duration.minute': '{count} Min.',
  'alerts.assistant.duration.minutes': '{count} Min.',
  'alerts.assistant.duration.unknown': 'unbekannte Dauer',
  'alerts.assistant.level.critical': 'Kritisch',
  'alerts.assistant.level.info': 'Info',
  'alerts.assistant.level.warning': 'Warnung',
  'alerts.assistant.locked.proRequired':
    'Pro erforderlich, um Pulse Assistant zu Warnmeldungen zu fragen',
  'alerts.assistant.locked.unavailable':
    'Pulse Assistant-Warnmeldungshilfe ist fuer diese Warnmeldung nicht verfuegbar',
  'alerts.assistant.safetyNote':
    'Diagnosen und Behebungen erfordern die Freigabe durch einen Operator.',
  'alerts.assistant.sourceLabel': 'Pulse Alerts',
  'alerts.assistant.statusLabel': '{level}-Warnmeldung · Aktiv seit {duration}',
  'alerts.assistant.subject': '{level} {alertType} auf {resourceName}',
  'alerts.assistant.title': 'Warnungsanalyse angehaengt',
  'alerts.assistant.unlockedTitle': 'Pulse Assistant zu dieser Warnmeldung fragen',
  'alerts.assistant.explain.menuLabel': 'Mit Assistant erklaeren',
  'alerts.assistant.explain.menuHint': 'Diese Warnmeldung als reinen Kontext verwenden',
  'alerts.assistant.patrol.chevron': 'Weitere Aktionen fuer diese Warnmeldung',
  'alerts.assistant.patrol.menuLabel': 'Patrol ermitteln lassen',
  'alerts.assistant.patrol.menuHint':
    'Eine gezielte Patrol-Pruefung fuer diese Ressource ausfuehren',
  'alerts.assistant.patrol.title': 'Patrol diese Warnmeldung untersuchen lassen',
  'alerts.assistant.patrol.triggered': 'Patrol untersucht {resourceName}',
  'alerts.assistant.patrol.noResource': 'Diese Warnmeldung hat keine zu untersuchende Ressource',
  'alerts.assistant.patrol.failed': 'Patrol-Pruefung konnte nicht gestartet werden',
  'alerts.nav.ariaLabel': 'Warnmeldungsnavigation',
  'alerts.nav.collapseSidebar': 'Seitenleiste einklappen',
  'alerts.nav.expandSidebar': 'Seitenleiste ausklappen',
  'alerts.nav.title': 'Warnmeldungen',
  'alerts.nav.toggleAlerts': 'Externe Benachrichtigungen umschalten',
  'alerts.overview.action.acknowledge': 'Bestaetigen',
  'alerts.overview.action.acknowledgeAll': 'Alle bestaetigen ({count})',
  'alerts.overview.action.acknowledging': 'Wird bestaetigt...',
  'alerts.overview.action.hideAcknowledged': 'Bestaetigte ausblenden',
  'alerts.overview.action.hideTimeline': 'Zeitleiste ausblenden',
  'alerts.overview.action.processing': 'Wird verarbeitet...',
  'alerts.overview.action.showAcknowledged': 'Bestaetigte anzeigen',
  'alerts.overview.action.timeline': 'Zeitleiste',
  'alerts.overview.action.unacknowledge': 'Bestaetigung aufheben',
  'alerts.overview.acknowledgedBadge': 'Bestaetigt',
  'alerts.overview.empty.description':
    'Warnmeldungen erscheinen hier, wenn Schwellwerte ueberschritten werden',
  'alerts.overview.empty.title': 'Keine aktiven Warnmeldungen',
  'alerts.overview.filteredEmpty.all': 'Keine aktiven Warnmeldungen',
  'alerts.overview.filteredEmpty.unacknowledged': 'Keine unbestaetigten Warnmeldungen',
  'alerts.overview.nodePrefix': 'auf {node}',
  'alerts.overview.notification.acknowledged': 'Warnmeldung bestaetigt',
  'alerts.overview.notification.acknowledgeFailed': 'Warnmeldung konnte nicht bestaetigt werden',
  'alerts.overview.notification.bulkFailure.singular':
    '{count} Warnmeldung konnte nicht bestaetigt werden.',
  'alerts.overview.notification.bulkFailure.plural':
    '{count} Warnmeldungen konnten nicht bestaetigt werden.',
  'alerts.overview.notification.bulkFailureGeneric':
    'Warnmeldungen konnten nicht bestaetigt werden',
  'alerts.overview.notification.bulkSuccess.singular': '{count} Warnmeldung bestaetigt.',
  'alerts.overview.notification.bulkSuccess.plural': '{count} Warnmeldungen bestaetigt.',
  'alerts.overview.notification.restored': 'Warnmeldung wiederhergestellt',
  'alerts.overview.notification.restoreFailed': 'Warnmeldung konnte nicht wiederhergestellt werden',
  'alerts.overview.paused.description':
    'Schalten Sie Warnmeldungen ein, um die Ueberwachung fortzusetzen und Konfigurationstabs freizugeben',
  'alerts.overview.paused.title': 'Warnmeldungen sind pausiert',
  'alerts.overview.section.activeAlerts': 'Aktive Warnmeldungen',
  'alerts.overview.startedAt': 'Gestartet: {startedAt}',
  'alerts.overview.stats.acknowledged': 'Bestaetigt',
  'alerts.overview.stats.triggered24h': 'Ausgeloest (24h)',
  'alerts.overview.stats.workloadOverrides': 'Workload-Ausnahmen',
  'alerts.page.default.description':
    'Pruefen Sie aktive Vorfaelle, Warnungsverlauf, Schwellwerte, Benachrichtigungen und Zeitplaene.',
  'alerts.page.default.title': 'Warnmeldungen',
  'alerts.page.destinations.description':
    'Leiten Sie Warnmeldungen an E-Mail-, Apprise-, Webhook- und Mobile-Push-Ziele weiter.',
  'alerts.page.destinations.title': 'Benachrichtigungen',
  'alerts.page.history.description':
    'Suchen Sie fruehere Warnmeldungen, pruefen Sie Vorfall-Zeitleisten und beobachten Sie Warnungshaeufigkeit ueber Zeit.',
  'alerts.page.history.title': 'Warnungsverlauf',
  'alerts.page.overview.description':
    'Pruefen Sie aktive Vorfaelle und die aktuelle Warnungsabdeckung der ueberwachten Ressourcen.',
  'alerts.page.overview.title': 'Warnmeldungsuebersicht',
  'alerts.page.schedule.description':
    'Definieren Sie Ruhezeiten, Gruppierung, Abklingzeiten, Wiederherstellung und Eskalation fuer Warnmeldungen.',
  'alerts.page.schedule.title': 'Wartungszeitplan',
  'alerts.page.thresholds.description':
    'Passen Sie Schwellwerte und gezielte Ausnahmen fuer Infrastruktur, Workloads und Integrationen an.',
  'alerts.page.thresholds.title': 'Warnungsschwellwerte',
  'alerts.tabs.disabledTitle':
    'Aktivieren Sie Warnmeldungen, um diese Einstellung zu konfigurieren',
  'alerts.tabs.group.configuration': 'Konfiguration',
  'alerts.tabs.group.status': 'Status',
  'alerts.tabs.destinations': 'Benachrichtigungen',
  'alerts.tabs.history': 'Verlauf',
  'alerts.tabs.overview': 'Uebersicht',
  'alerts.tabs.schedule': 'Zeitplan',
  'alerts.tabs.thresholds': 'Schwellwerte',
  'alerts.timeline.acknowledged': 'bestaetigt',
  'alerts.timeline.closedAt': 'geschlossen {closedAt}',
  'alerts.timeline.empty': 'Noch keine Zeitleistenereignisse.',
  'alerts.timeline.failure': 'Zeitleiste konnte nicht geladen werden.',
  'alerts.timeline.filterEmpty':
    'Keine Zeitleistenereignisse entsprechen den ausgewaehlten Filtern.',
  'alerts.timeline.filterLabel.compact': 'Filter',
  'alerts.timeline.filterLabel.panel': 'Ereignisse filtern:',
  'alerts.timeline.heading': 'Vorfall',
  'alerts.timeline.loading': 'Zeitleiste wird geladen...',
  'alerts.timeline.event.alertAcknowledged': 'Bestaetigt',
  'alerts.timeline.event.alertFired': 'Ausgeloest',
  'alerts.timeline.event.alertResolved': 'Behoben',
  'alerts.timeline.event.alertUnacknowledged': 'Unbestaetigt',
  'alerts.timeline.event.aiAnalysis': 'Patrol',
  'alerts.timeline.event.command': 'Befehl',
  'alerts.timeline.event.note': 'Notiz',
  'alerts.timeline.event.runbook': 'Runbook',
  'alerts.timeline.noteLabel': 'Vorfallsnotiz',
  'alerts.timeline.notePlaceholder': 'Notiz fuer diesen Vorfall hinzufuegen...',
  'alerts.timeline.openedAt': 'geoeffnet {openedAt}',
  'alerts.timeline.quickFilter.all': 'Alle',
  'alerts.timeline.quickFilter.none': 'Keine',
  'alerts.timeline.retry': 'Erneut versuchen',
  'alerts.timeline.saveNote': 'Notiz speichern',
  'alerts.timeline.savingNote': 'Wird gespeichert...',
  'alerts.timeline.unavailable': 'Keine Vorfall-Zeitleiste verfuegbar.',
  'runtimeHome.openingWorkspace': 'Arbeitsbereich wird geoeffnet...',
  'setup.completion.action.addInfrastructure': 'Infrastruktur hinzufuegen',
  'setup.completion.action.copyAdminToken': 'Admin-API-Token kopieren',
  'setup.completion.action.copyPassword': 'Passwort kopieren',
  'setup.completion.action.downloadCredentials': 'Zugangsdaten herunterladen',
  'setup.completion.action.installAgent': 'Pulse Agent installieren',
  'setup.completion.action.openInfrastructure': 'Infrastruktur oeffnen',
  'setup.completion.connectedSummary.singular': 'Verbunden ({count} System)',
  'setup.completion.connectedSummary.plural': 'Verbunden ({count} Systeme)',
  'setup.completion.credentials.apiTokenLabel': 'Admin-API-Token',
  'setup.completion.credentials.badge': 'Waehrend der Einrichtung angezeigt',
  'setup.completion.credentials.continuation.empty': 'Infrastruktur hinzufuegen.',
  'setup.completion.credentials.continuation.connected':
    'Infrastruktur oder Infrastruktur hinzufuegen.',
  'setup.completion.credentials.description':
    'Speichern Sie Admin-Anmeldung und API-Token, bevor Sie diesen Bildschirm verlassen, und fahren Sie dann mit {destination} fort',
  'setup.completion.credentials.passwordLabel': 'Passwort',
  'setup.completion.credentials.title': 'Zugangsdaten jetzt speichern',
  'setup.completion.credentials.usernameLabel': 'Benutzername',
  'setup.completion.download.content':
    'Pulse-Zugangsdaten\n==================\nErstellt: {generatedAt}\n\nWeb-Anmeldung:\n----------\nURL: {baseUrl}\nBenutzername: {username}\nPasswort: {password}\n\nAdmin-API-Token:\n----------------\n{apiToken}\n\nInfrastruktur:\n---------------\n{infrastructureUrl}\n\nVerwenden Sie Infrastruktur hinzufuegen, um eine Plattform-API, Pulse Agent oder beides\nfuer das erste System auszuwaehlen, das Pulse ueberwachen soll.\n\nBewahren Sie diese Zugangsdaten sicher auf!\n',
  'setup.completion.hero.connected.description.agent':
    'Ihr Admin-Konto ist bereit und Pulse empfaengt bereits Telemetrie. Oeffnen Sie Infrastruktur, um das erste System zu pruefen, und kehren Sie zu Infrastruktur hinzufuegen zurueck, wenn Sie eine weitere Pulse Agent- oder Plattform-API-Quelle wollen.',
  'setup.completion.hero.connected.description.api':
    'Ihr Admin-Konto ist bereit und Pulse empfaengt bereits Telemetrie. Oeffnen Sie Infrastruktur, um das erste System zu pruefen, und kehren Sie zu Infrastruktur hinzufuegen zurueck, wenn Sie eine weitere Plattform-API- oder Pulse Agent-Quelle wollen.',
  'setup.completion.hero.connected.description.both':
    'Ihr Admin-Konto ist bereit und Pulse empfaengt bereits Telemetrie. Oeffnen Sie Infrastruktur, um das erste System zu pruefen, und kehren Sie zu Infrastruktur hinzufuegen zurueck, wenn Sie eine weitere Plattform-API- oder Agent-Quelle wollen.',
  'setup.completion.hero.connected.title': 'Erstes ueberwachtes System verbunden',
  'setup.completion.hero.empty.description':
    'Ihr Admin-Konto ist bereit. Waehlen Sie als Naechstes, wie das erste System in das einheitliche Infrastrukturmodell gelangt: Plattform-API-Inventar, Pulse Agent-Telemetrie oder beides.',
  'setup.completion.hero.empty.title': 'Erste Infrastrukturquelle waehlen',
  'setup.completion.nextStep.ariaLabel': 'Naechster Einrichtungsschritt',
  'setup.completion.nextStep.badge': 'Empfohlener naechster Schritt',
  'setup.completion.nextStep.detail.agent':
    'Infrastruktur hinzufuegen bleibt fuer weitere Pulse Agent-Systeme oder Plattform-API-Inventar verfuegbar, wenn eine Plattform die Umgebung verwaltet.',
  'setup.completion.nextStep.detail.api':
    'Infrastruktur hinzufuegen bleibt fuer weitere API-gestuetzte Systeme oder Pulse Agent-Telemetrie verfuegbar, wenn ein System lokale Knotenabdeckung braucht.',
  'setup.completion.nextStep.detail.both':
    'Infrastruktur hinzufuegen bleibt jederzeit verfuegbar, wenn Sie dieses erste System um eine weitere API-Quelle, Agent-Quelle oder beides erweitern wollen.',
  'setup.completion.nextStep.detail.empty':
    'Beginnen Sie mit einer Plattform-API, wenn eine Plattform die Umgebung verwaltet. Installieren Sie Pulse Agent, wenn das System selbst lokale Knotentelemetrie melden soll.',
  'setup.completion.nextStep.label': 'Naechster Schritt',
  'setup.completion.nextStep.summary.connected.singular':
    'Oeffnen Sie Infrastruktur, um Ihr erstes verbundenes System zu pruefen.',
  'setup.completion.nextStep.summary.connected.plural':
    'Oeffnen Sie Infrastruktur, um Ihre verbundenen Systeme zu pruefen.',
  'setup.completion.nextStep.summary.empty':
    'Oeffnen Sie Infrastruktur hinzufuegen, um eine Plattform-API, Pulse Agent oder beides zu waehlen.',
  'setup.completion.nextStep.title.connected': 'Infrastruktur oeffnen',
  'setup.completion.nextStep.title.empty': 'Strategie fuer erste Quelle waehlen',
  'setup.completion.resource.unknownName': 'Unbekannt',
  'setup.completion.sourceOptions.agent.description':
    'Knotenlokale Telemetrie fuer eigenstaendige Hosts, Dienste, Docker und Kubernetes.',
  'setup.completion.sourceOptions.agent.title': 'Pulse Agent',
  'setup.completion.sourceOptions.both.description':
    'Kombinieren Sie Plattforminventar mit Agent-Telemetrie, wenn vollstaendige Abdeckung wichtig ist.',
  'setup.completion.sourceOptions.both.title': 'Beides verwenden',
  'setup.completion.sourceOptions.platformApi.description':
    'Inventar und Zustand aus Proxmox, TrueNAS, VMware, PBS oder PMG.',
  'setup.completion.sourceOptions.platformApi.title': 'Plattform-API',
  'setup.completion.sourceOptions.title': 'Quellenoptionen',
  'setup.progress.ariaLabel': 'Einrichtungsfortschritt',
  'setup.progress.state.completed': ', abgeschlossen',
  'setup.progress.state.current': ', aktuell',
  'setup.progress.stepAriaLabel': 'Schritt {index}: {step}{state}',
  'setup.security.action.back': 'Zurueck',
  'setup.security.action.createAccount': 'Konto erstellen und fortfahren',
  'setup.security.action.settingUp': 'Einrichtung laeuft...',
  'setup.security.customPassword': 'Eigenes Passwort',
  'setup.security.description':
    'Der naechste Schritt ist die Auswahl der ersten Infrastrukturquelle.',
  'setup.security.error.passwordMismatch': 'Passwoerter stimmen nicht ueberein',
  'setup.security.error.passwordRequired': 'Bitte geben Sie ein Passwort ein',
  'setup.security.error.passwordTooShort': 'Das Passwort muss mindestens 12 Zeichen haben',
  'setup.security.error.setupFailed': 'Einrichtung fehlgeschlagen: {error}',
  'setup.security.generatedPasswordHelp':
    'Ein sicheres 20-Zeichen-Passwort wird erzeugt und auf dem naechsten Bildschirm angezeigt.',
  'setup.security.label.confirmPassword': 'Passwort bestaetigen',
  'setup.security.label.password': 'Passwort',
  'setup.security.label.username': 'Benutzername',
  'setup.security.minimumPasswordHelp': 'Mindestens 12 Zeichen.',
  'setup.security.nextScreen.itemApiToken': 'Ein Admin-API-Token fuer Automatisierung',
  'setup.security.nextScreen.itemCredentials': 'Ihr Benutzername und Passwort',
  'setup.security.nextScreen.saveOnce':
    'Speichern Sie sie vor dem Fortfahren; sie werden nur einmal angezeigt.',
  'setup.security.nextScreen.title': 'Auf dem naechsten Bildschirm',
  'setup.security.passwordMode.autoGenerate': 'Automatisch erzeugen',
  'setup.security.placeholder.password': 'Passwort (mind. 12 Zeichen)',
  'setup.security.placeholder.username': 'admin',
  'setup.security.showPassword.hide': 'Ausblenden',
  'setup.security.showPassword.show': 'Anzeigen',
  'setup.security.title': 'Admin-Konto erstellen',
  'setup.step.firstSource': 'Erste Quelle',
  'setup.step.security': 'Sicherheit',
  'setup.step.unlockServer': 'Server entsperren',
  'setup.wizard.ariaLabel': 'Pulse-Einrichtungsassistent',
  'setup.welcome.action.continueSecurity': 'Weiter zu Sicherheit',
  'setup.welcome.action.verifyToken': 'Bootstrap-Token pruefen',
  'setup.welcome.action.verifyingToken': 'Bootstrap-Token wird geprueft...',
  'setup.welcome.copyCommandTitle': 'Befehl kopieren',
  'setup.welcome.deploymentHint.choose':
    'Fuehren Sie den Befehl aus, der zur Pulse-Installation passt. Der Server legt vor dem Entsperren der Einrichtung keine Bereitstellungsdetails offen.',
  'setup.welcome.deploymentHint.containerized':
    'Pulse scheint in einer containerisierten Umgebung zu laufen. Fuehren Sie den Befehl auf dem Host aus, der den Container verwaltet, damit Sie das einmalige Setup-Token ausgeben koennen.',
  'setup.welcome.deploymentHint.direct':
    'Fuehren Sie den Befehl direkt in einer Shell auf dem Pulse-Server aus, um das einmalige Setup-Token auszugeben.',
  'setup.welcome.deploymentHint.dockerNamed':
    'Pulse scheint in Docker als Container "{containerName}" zu laufen. Fuehren Sie den Befehl auf dem Docker-Host aus, um das einmalige Setup-Token aus diesem Container auszugeben.',
  'setup.welcome.deploymentHint.dockerUnnamed':
    'Pulse scheint in Docker zu laufen. Fuehren Sie den Befehl auf dem Docker-Host aus und ersetzen Sie <pulse-container> durch den Namen des laufenden Pulse-Containers.',
  'setup.welcome.deploymentHint.lxc':
    'Pulse scheint in LXC-Container {ctid} zu laufen. Fuehren Sie den Befehl auf dem Proxmox-Host aus, um in diesen Container zu wechseln und das einmalige Setup-Token auszugeben.',
  'setup.welcome.deploymentLabel.containerized': 'Containerisierte Bereitstellung',
  'setup.welcome.deploymentLabel.direct': 'Direkte Host-Installation',
  'setup.welcome.deploymentLabel.docker': 'Docker-Bereitstellung',
  'setup.welcome.deploymentLabel.lxc': 'LXC-Container',
  'setup.welcome.error.copyCommandFailed': 'Befehl konnte nicht kopiert werden',
  'setup.welcome.error.invalidBootstrapToken':
    'Ungueltiges Bootstrap-Token. Bitte pruefen und erneut versuchen.',
  'setup.welcome.error.invalidBootstrapTokenResponse': 'Ungueltiges Bootstrap-Token',
  'setup.welcome.error.missingBootstrapToken': 'Bitte geben Sie das Bootstrap-Token ein',
  'setup.welcome.error.snapshotPaste':
    'Das sieht nach dem verschluesselten Inhalt der Datei .bootstrap_token aus, nicht nach dem rohen Setup-Token. Fuehren Sie den passenden Befehl aus und fuegen Sie die ausgegebene Token-Zeichenfolge ein.',
  'setup.welcome.hero.coverage':
    'Verbinden Sie eine Plattform-API, installieren Sie Pulse Agent oder verwenden Sie beides fuer vollstaendige Abdeckung.',
  'setup.welcome.hero.step.admin': 'Admin-Konto erstellen',
  'setup.welcome.hero.step.source': 'Erste Quelle waehlen',
  'setup.welcome.hero.step.unlock': 'Diesen Pulse-Server entsperren',
  'setup.welcome.hero.stepsIntro': 'Drei Schritte:',
  'setup.welcome.hero.stepsThen': 'dann',
  'setup.welcome.hero.title': 'Willkommen bei Pulse',
  'setup.welcome.placeholder.bootstrapToken': 'Bootstrap-Token einfuegen',
  'setup.welcome.success.commandCopied': 'Befehl in Zwischenablage kopiert',
  'setup.welcome.telemetryNotice.description':
    'Ausgehende Nutzungstelemetrie ist standardmaessig aktiviert. Pulse sendet einen verzoegerten Start-Ping und einen taeglichen Heartbeat mit einer rotierenden pseudonymen Installations-ID, Release-/Runtime-Details, aggregierten Zaehlern und Funktionsflags. Um sie vor jedem Ping zu deaktivieren, setzen Sie PULSE_TELEMETRY=false, bevor Sie Pulse starten; spaeter koennen Sie sie auch in den Einstellungen ausschalten.',
  'setup.welcome.telemetryNotice.detailsLink': 'Details',
  'setup.welcome.telemetryNotice.title': 'Nutzungstelemetrie ist standardmaessig aktiviert',
  'setup.welcome.tokenHelp.afterVerify':
    'Nachdem Pulse dieses Token geprueft hat, erstellen Sie im naechsten Schritt das Admin-Konto fuer diesen Server.',
  'setup.welcome.tokenHelp.docker':
    'Dieses einmalige Bootstrap-Token entsperrt nur die Ersteinrichtung. Fuehren Sie den obigen Befehl aus und fuegen Sie die ausgegebene Token-Zeichenfolge ein. Nach der Pruefung erstellen Sie das Admin-Konto, und Pulse erzeugt das langlebige API-Token separat.',
  'setup.welcome.tokenHelp.host':
    'Dieses einmalige Bootstrap-Token entsperrt nur die Ersteinrichtung auf diesem Pulse-Server. Fuehren Sie den obigen Befehl aus und fuegen Sie die ausgegebene Token-Zeichenfolge ein. Es ist nicht Ihr Admin-Passwort und nicht das API-Token, das Sie nach der Einrichtung verwenden.',
  'setup.welcome.tokenHelp.generic':
    'Dieses einmalige Bootstrap-Token entsperrt nur die Ersteinrichtung auf diesem Pulse-Server. Fuehren Sie den passenden Befehl aus und fuegen Sie die ausgegebene Zeichenfolge ein. Es ist weder Ihr Admin-Passwort noch ein langlebiges API-Token.',
  'setup.welcome.tokenHelp.title': 'Was dieses Token macht',
  'setup.welcome.unlockTitle': 'Einrichtung entsperren',
  'settings.general.appearance.title': 'Darstellung',
  'settings.general.fullWidth.description':
    'Nutzen Sie auf grossen Monitoren die gesamte verfuegbare Bildschirmbreite.',
  'settings.general.fullWidth.title': 'Volle Breite',
  'settings.general.docker.envHint': 'Kann auch ueber Umgebungsvariable gesetzt werden:',
  'settings.general.docker.section.description':
    'Steuern Sie, wie {sourceLabel}-Updateaktionen in Pulse angezeigt werden.',
  'settings.general.docker.section.title': '{sourceLabel}-Updates',
  'settings.general.docker.toggle.description':
    'Wenn aktiviert, werden {sourceLabel}-Aktionen "Update" in Pulse ausgeblendet. Die Update-Erkennung laeuft weiter, sodass verfuegbare Updates sichtbar bleiben.',
  'settings.general.docker.toggle.title': 'Update-Schaltflaechen ausblenden',
  'settings.general.language.ariaLabel': 'App-Sprache',
  'settings.general.language.description':
    'Verwenden Sie diese Sprache fuer die App-Oberflaeche. Befehle, Ressourcennamen und API-Felder bleiben unveraendert.',
  'settings.general.language.title': 'Sprache',
  'settings.general.monitoringCadence.current': 'Aktueller Takt: {seconds} Sekunden ({duration})',
  'settings.general.monitoringCadence.custom.description':
    'Sekunden eingeben ({min}-{max}). Gilt fuer alle Cluster.',
  'settings.general.monitoringCadence.custom.title': 'Benutzerdefiniertes Abfrageintervall',
  'settings.general.monitoringCadence.description':
    'Kuerzere Intervalle liefern nahezu Echtzeit-Aktualisierungen auf Kosten hoeherer API- und CPU-Nutzung auf jedem Knoten. Waehlen Sie ein laengeres Intervall, um die Last auf ausgelasteten Clustern zu reduzieren.',
  'settings.general.monitoringCadence.duration.minute': '{count} Minute',
  'settings.general.monitoringCadence.duration.minutes': '{count} Minuten',
  'settings.general.monitoringCadence.duration.underMinute': 'unter einer Minute',
  'settings.general.monitoringCadence.envLocked':
    'Wird ueber Umgebungsvariable {envVar} verwaltet.',
  'settings.general.monitoringCadence.preset.balanced': 'Ausgewogen ({duration})',
  'settings.general.monitoringCadence.preset.custom': 'Benutzerdefiniert',
  'settings.general.monitoringCadence.preset.low': 'Niedrig ({duration})',
  'settings.general.monitoringCadence.preset.realtime': 'Echtzeit ({duration})',
  'settings.general.monitoringCadence.preset.veryLow': 'Sehr niedrig ({duration})',
  'settings.general.monitoringCadence.section.description':
    'Steuern Sie, wie oft Pulse Proxmox VE-Knoten abfragt.',
  'settings.general.monitoringCadence.section.title': 'Monitoring-Takt',
  'settings.general.temperature.description': 'Temperaturen in Celsius oder Fahrenheit anzeigen.',
  'settings.general.temperature.title': 'Temperatureinheit',
  'settings.general.telemetry.copyJson': 'JSON kopieren',
  'settings.general.telemetry.description':
    'Helfen Sie, Pulse zu verbessern, indem Sie ausgehende Nutzungsdaten teilen: eine rotierende pseudonyme Installations-ID, normalisierte Release-Identitaet, Laufzeitplattform, grobe Kategorien fuer Bereitstellungsart und Lebenszyklus, aggregierte Ressourcen- und Ergebniszahlen, grobe Funktionsflags sowie inhaltsfreie Nutzungszaehler fuer Patrol, Assistant und Capability-APIs. Der Payload enthaelt keine Hostnamen, Zugangsdaten, Infrastrukturkennungen, URLs, Pfade, Gebietsschema, Browser-Ereignisse, Prompts, Chatnachrichten, Befehlstexte, Aktionsausgaben, Token-Werte, Namen, E-Mail-Adressen oder IP-Adressen. Telemetriezeilen werden bis zu 90 Tage aufbewahrt, und Anfrage-IP-Adressen werden nur kurzzeitig fuer Rate-Limiting verwendet und nicht in Telemetriezeilen gespeichert.',
  'settings.general.telemetry.disabledPreview':
    'Telemetrie ist derzeit deaktiviert. Diese Vorschau zeigt den Payload, den Pulse senden wuerde, wenn Sie sie aktivieren.',
  'settings.general.telemetry.fullDetails': 'Details',
  'settings.general.telemetry.notice.description':
    'Pulse fuegt jetzt grobe Signale zu Bereitstellung, Lebenszyklus, Bestandsgroesse sowie aggregierte Alarm- und Benachrichtigungsergebnisse hinzu, wenn ausgehende Nutzungstelemetrie aktiviert ist. Persoenliche Daten, Infrastruktur-IDs, Inhalte, Browser-Ereignisse und Clickstream-Daten bleiben ausgeschlossen.',
  'settings.general.telemetry.notice.disable': 'Telemetrie deaktivieren',
  'settings.general.telemetry.notice.dismissLabel': 'Hinweis zum Telemetrie-Payload schliessen',
  'settings.general.telemetry.notice.dismissTitle': 'Dauerhaft schliessen',
  'settings.general.telemetry.notice.preview': 'Payload anzeigen',
  'settings.general.telemetry.notice.privacy': 'Datenschutzdetails',
  'settings.general.telemetry.notice.title': 'Telemetrie-Payload aktualisiert',
  'settings.general.telemetry.payloadAriaLabel': 'Telemetrie-Payload-Vorschau',
  'settings.general.telemetry.payloadTitle': 'Aktueller Heartbeat-Payload',
  'settings.general.telemetry.previewPayload': 'Payload anzeigen',
  'settings.general.telemetry.refreshPayload': 'Payload aktualisieren',
  'settings.general.telemetry.resetId': 'ID zuruecksetzen',
  'settings.general.telemetry.section.description':
    'Steuern Sie ausgehende Nutzungstelemetrie von dieser Pulse-Instanz.',
  'settings.general.telemetry.section.title': 'Nutzungsdaten und Datenschutz',
  'settings.general.telemetry.title': 'Ausgehende Nutzungstelemetrie',
  'settings.general.theme.description': 'Waehlen Sie hell, dunkel oder die Systemeinstellung.',
  'settings.general.theme.option.dark': 'Dunkel',
  'settings.general.theme.option.light': 'Hell',
  'settings.general.theme.option.system': 'System',
  'settings.general.theme.title': 'Designpraeferenz',
  'settings.header.api.description':
    'Erstellen und verwalten Sie begrenzte Pulse-Tokens fuer Agents, Automatisierung und externe Integrationen.',
  'settings.header.api.title': 'API-Zugriff',
  'settings.header.infrastructure.description':
    'Fuegen Sie Infrastruktur hinzu, finden Sie sie automatisch, und pruefen Sie, was Pulse ueberwacht.',
  'settings.header.infrastructure.readOnlyDescription':
    'Pruefen Sie die aktuell ueberwachten Hauptsysteme und den Reporting-Status. Einrichtungsänderungen bleiben in dieser schreibgeschuetzten Sitzung nicht verfuegbar.',
  'settings.header.infrastructure.title': 'Infrastruktur',
  'settings.header.monitoringAvailability.description':
    'Ueberwachen Sie Endgeraete und Dienste nur ueber Ping-, TCP- und HTTP-Pruefungen.',
  'settings.header.monitoringAvailability.title': 'Verfuegbarkeitspruefungen',
  'settings.header.organizationAccess.description':
    'Verwalten Sie Einladungen, Mitgliederrollen und Eigentumsuebertragungen der Organisation.',
  'settings.header.organizationAccess.title': 'Organisationszugriff',
  'settings.header.organizationBilling.description':
    'Pruefen Sie Organisationsplan, geltende Nutzungsrichtlinien und Abonnementstatus fuer bezahlten Zugriff.',
  'settings.header.organizationBilling.title': 'Abrechnung & Nutzung',
  'settings.header.organizationBillingAdmin.description':
    'Pruefen und verwalten Sie den Abrechnungsstatus aller Mandanten (nur Hosted-Modus).',
  'settings.header.organizationBillingAdmin.title': 'Abrechnungsadmin',
  'settings.header.organizationOverview.description':
    'Pruefen Sie Organisationsdaten, Mitgliederumfang und Eigentum.',
  'settings.header.organizationOverview.title': 'Organisationsuebersicht',
  'settings.header.organizationSharing.description':
    'Teilen Sie Ressourcen zwischen Organisationen mit begrenztem Zugriff.',
  'settings.header.organizationSharing.title': 'Organisationsfreigabe',
  'settings.header.securityAudit.description':
    'Sehen Sie Sicherheitsereignisse, Anmeldeversuche und Konfigurationsaenderungen.',
  'settings.header.securityAudit.title': 'Audit-Protokoll',
  'settings.header.securityAuth.description':
    'Verwalten Sie passwortbasierte Authentifizierung und Zugangsdatenrotation.',
  'settings.header.securityAuth.title': 'Authentifizierung',
  'settings.header.securityDataHandling.description':
    'Sehen Sie, welche ueberwachten Ressourcendetails zusammengefasst werden duerfen, lokal bleiben muessen oder redigiert sind.',
  'settings.header.securityDataHandling.title': 'Ressourcenschutz',
  'settings.header.securityOverview.description':
    'Sehen Sie Ihre Sicherheitslage auf einen Blick und ueberwachen Sie den Authentifizierungsstatus.',
  'settings.header.securityOverview.title': 'Sicherheitsuebersicht',
  'settings.header.securityRoles.description':
    'Definieren Sie eigene Rollen und verwalten Sie granulare Berechtigungen fuer Benutzer und Tokens.',
  'settings.header.securityRoles.title': 'Rollen',
  'settings.header.securitySso.description': 'Konfigurieren Sie OIDC- und SAML-Identity-Provider.',
  'settings.header.securitySso.title': 'Single-Sign-On-Anbieter',
  'settings.header.securityUsers.description':
    'Weisen Sie Benutzern Rollen zu und sehen Sie effektive Berechtigungen in Ihrer Infrastruktur.',
  'settings.header.securityUsers.title': 'Benutzerzugriff',
  'settings.header.securityWebhooks.description':
    'Konfigurieren Sie die Echtzeit-Zustellung von Audit-Ereignissen an externe Systeme.',
  'settings.header.securityWebhooks.title': 'Audit-Webhooks',
  'settings.header.systemAi.description':
    'Konfigurieren Sie Anbieter, Standardmodelle, Anbieterzustand, Budget und Nutzung fuer Pulse Intelligence.',
  'settings.header.systemAi.title': 'Anbieter & Modelle',
  'settings.header.systemAiAssistant.description':
    'Konfigurieren Sie Chatverhalten, Aktionsrechte, Sitzungen und externe Agent-Verbindungen (MCP) des Assistant.',
  'settings.header.systemAiAssistant.title': 'Assistant',
  'settings.header.systemAiDiscovery.description':
    'Konfigurieren Sie den KI-gestuetzten Service-Kontext fuer Assistant und Patrol. Infrastruktur-Erkennung und Onboarding bleiben unter Infrastruktur.',
  'settings.header.systemAiDiscovery.title': 'Service-Kontext',
  'settings.header.systemAiPatrol.description':
    'Legen Sie fest, wann Patrol laeuft, was Patrol startet und welches Modell verwendet wird.',
  'settings.header.systemAiPatrol.title': 'Patrol',
  'settings.header.systemBilling.description': 'Plan, Lizenz und Patrol-Modus fuer diese Instanz.',
  'settings.header.systemBilling.title': 'Plaene & Abrechnung',
  'settings.header.systemGeneral.description':
    'Verwalten Sie Darstellung, Layout und Standard-Monitoring-Takt.',
  'settings.header.systemGeneral.title': 'Allgemein',
  'settings.header.systemNetwork.description':
    'Konfigurieren Sie oeffentliche URL, CORS, Einbettung und Netzwerkgrenzen fuer Webhooks.',
  'settings.header.systemNetwork.title': 'Netzwerk',
  'settings.header.systemRecovery.description':
    'Verwalten Sie Backup-/Snapshot-Abfragen sowie Export- und Importablaeufe der Konfiguration.',
  'settings.header.systemRecovery.title': 'Wiederherstellung',
  'settings.header.systemRelay.description':
    'Behalten Sie Ihre Systeme von ueberall im Blick und erhalten Sie Alarm-Push-Benachrichtigungen ueber die Pulse-Mobile-App — ohne Portfreigaben oder VPN.',
  'settings.header.systemRelay.title': 'Remote-Zugriff',
  'settings.header.systemUpdates.description':
    'Verwalten Sie Versionspruefungen, Update-Kanaele und automatische Updates der Pulse-Server-Laufzeit. Agent-Updates bleiben unter Infrastruktur.',
  'settings.header.systemUpdates.title': 'Pulse-Server-Updates',
  'settings.header.supportDiagnostics.description':
    'Fuehren Sie Zustandspruefungen aus, validieren Sie Verbindungen und exportieren Sie Troubleshooting-Snapshots.',
  'settings.header.supportDiagnostics.title': 'Diagnose & Zustand',
  'settings.header.supportLogs.description':
    'Pruefen Sie den Live-Pulse-Logstream und laden Sie den erfassten Puffer fuer Supportarbeit herunter.',
  'settings.header.supportLogs.title': 'Systemprotokolle',
  'settings.header.supportReporting.description':
    'Exportieren Sie Inventardaten und erstellen Sie Leistungsberichte aus der kanonischen Einstellungsoberflaeche.',
  'settings.header.supportReporting.title': 'Daten & Berichte',
  'settings.nav.group.infrastructure': 'Infrastruktur',
  'settings.nav.group.monitoring': 'Monitoring',
  'settings.nav.group.organization': 'Organisation',
  'settings.nav.group.pulseIntelligence': 'Pulse Intelligence',
  'settings.nav.group.security': 'Sicherheit',
  'settings.nav.group.support': 'Support',
  'settings.nav.group.system': 'System',
  'settings.nav.item.apiAccess': 'API-Zugriff',
  'settings.nav.item.auditLog': 'Audit-Protokoll',
  'settings.nav.item.auditWebhooks': 'Audit-Webhooks',
  'settings.nav.item.authentication': 'Authentifizierung',
  'settings.nav.item.availabilityChecks': 'Verfuegbarkeitspruefungen',
  'settings.nav.item.billing': 'Abrechnung',
  'settings.nav.item.billingAdmin': 'Abrechnungsadmin',
  'settings.nav.item.dataReports': 'Daten & Berichte',
  'settings.nav.item.diagnosticsHealth': 'Diagnose & Zustand',
  'settings.nav.item.assistant': 'Assistant',
  'settings.nav.item.discovery': 'Service-Kontext',
  'settings.nav.item.general': 'Allgemein',
  'settings.nav.item.infrastructure': 'Infrastruktur',
  'settings.nav.item.network': 'Netzwerk',
  'settings.nav.item.organizationAccess': 'Zugriff',
  'settings.nav.item.organizationOverview': 'Uebersicht',
  'settings.nav.item.patrol': 'Patrol',
  'settings.nav.item.plans': 'Plaene & Abrechnung',
  'settings.nav.item.providerModels': 'Anbieter & Modelle',
  'settings.nav.item.recovery': 'Wiederherstellung',
  'settings.nav.item.remoteAccess': 'Remote-Zugriff',
  'settings.nav.item.resourcePrivacy': 'Ressourcenschutz',
  'settings.nav.item.roles': 'Rollen',
  'settings.nav.item.securityOverview': 'Sicherheitsuebersicht',
  'settings.nav.item.sharing': 'Freigabe',
  'settings.nav.item.singleSignOn': 'Single Sign-On',
  'settings.nav.item.systemLogs': 'Systemprotokolle',
  'settings.nav.item.updates': 'Pulse-Server-Updates',
  'settings.nav.item.users': 'Benutzer',
  'settings.shell.collapseSidebarLabel': 'Einstellungsnavigation einklappen',
  'settings.shell.configurationLoading': 'Konfiguration wird geladen...',
  'settings.shell.discardLabel': 'Verwerfen',
  'settings.shell.expandSidebarLabel': 'Einstellungsnavigation ausklappen',
  'settings.shell.loading': 'Einstellungen werden geladen...',
  'settings.shell.mobileBackLabel': 'Einstellungen',
  'settings.shell.navigationAriaLabel': 'Einstellungsnavigation',
  'settings.shell.navigationTitle': 'Einstellungen',
  'settings.shell.saveChangesLabel': 'Aenderungen speichern',
  'settings.shell.searchEmpty': 'Keine Einstellungen fuer "{query}" gefunden',
  'settings.shell.searchPlaceholder': 'Einstellungen suchen...',
  'settings.shell.unsavedDescription':
    'Ihre Aenderungen gehen verloren, wenn Sie diese Seite verlassen.',
  'settings.shell.unsavedTitle': 'Nicht gespeicherte Aenderungen',
} as const satisfies Partial<I18nCatalog>;
