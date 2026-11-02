import type { I18nCatalog } from './messages';

export const FR_MESSAGE_OVERRIDES = {
  'pricing.handoff.description.afterLink': '.',
  'pricing.handoff.description.beforeLink': 'Si la redirection ne démarre pas automatiquement,',
  'pricing.handoff.link.publicPricing': 'continuez vers la page publique des tarifs',
  'pricing.handoff.link.pulseAccount': 'continuez vers Pulse Account',
  'pricing.handoff.title.publicPricing': 'Redirection vers les tarifs',
  'pricing.handoff.title.pulseAccount': 'Redirection vers Pulse Account',
  'alerts.activation.switchLabel': 'Envoyer les notifications',
  'alerts.activation.toast.activated':
    'Notifications activées. Vous serez désormais averti lorsque des problèmes seront détectés.',
  'alerts.activation.toast.activateFailed':
    "Impossible d'activer les notifications. Veuillez réessayer.",
  'alerts.activation.toast.deactivated':
    "Notifications en pause. Pulse continuera de détecter et d'afficher les alertes actives.",
  'alerts.activation.toast.deactivateFailed':
    'Impossible de mettre les notifications en pause. Veuillez réessayer.',
  'alerts.investigate.failed': 'Impossible de lancer la vérification Patrol',
  'alerts.investigate.iconLabel': "Demander à Patrol d'examiner {alertType} sur {resourceName}",
  'alerts.investigate.iconLabelAt':
    "Demander à Patrol d'examiner {alertType} sur {resourceName}, {startedAt}",
  'alerts.investigate.label': 'Faire examiner par Patrol',
  'alerts.investigate.title': "Demander à Patrol d'examiner cette alerte",
  'alerts.investigate.triggered':
    'Patrol examine {resourceName}. Les résultats apparaissent sur la page Patrol.',
  'alerts.nav.ariaLabel': 'Navigation des alertes',
  'alerts.nav.collapseSidebar': 'Réduire la barre latérale',
  'alerts.nav.expandSidebar': 'Développer la barre latérale',
  'alerts.nav.title': 'Alertes',
  'alerts.overview.action.acknowledge': 'Acquitter',
  'alerts.overview.action.acknowledgeAll': 'Tout acquitter ({count})',
  'alerts.overview.action.acknowledging': 'Acquittement...',
  'alerts.overview.action.cancel': 'Annuler',
  'alerts.overview.action.hideAcknowledged': 'Masquer les acquittées',
  'alerts.overview.action.hideTimeline': 'Masquer la chronologie',
  'alerts.overview.action.moreActions': 'Plus',
  'alerts.overview.action.lessActions': 'Moins',
  'alerts.overview.action.processing': 'Traitement...',
  'alerts.overview.action.resume': 'Reprendre',
  'alerts.overview.action.showAcknowledged': 'Afficher les acquittées',
  'alerts.overview.action.timeline': 'Chronologie',
  'alerts.overview.action.snooze': 'Reporter',
  'alerts.overview.action.unacknowledge': "Annuler l'acquittement",
  'alerts.overview.group.acknowledge': 'Tout acquitter ({count})',
  'alerts.overview.group.hide': 'Masquer',
  'alerts.overview.group.linkedSignal': '+{count} signal lié',
  'alerts.overview.group.linkedSignals': '+{count} signaux liés',
  'alerts.overview.group.related': '+{count} en lien',
  'alerts.overview.synthesis.title.supported': "Cause d'infrastructure étayée",
  'alerts.overview.synthesis.title.observationSet': "Ensemble d'observations liées",
  'alerts.overview.synthesis.failure.runtime': 'Exécution',
  'alerts.overview.synthesis.failure.networkPath': 'Chemin réseau',
  'alerts.overview.synthesis.failure.applicationResponse': "Réponse de l'application",
  'alerts.overview.synthesis.failure.certificate': 'Certificat',
  'alerts.overview.synthesis.failure.dependency': 'Dépendance',
  'alerts.overview.synthesis.failure.evidenceCoverage': 'Couverture des éléments probants',
  'alerts.overview.synthesis.counts':
    '{affected} ressources concernées · {observations} observations',
  'alerts.overview.synthesis.review': 'Examiner les éléments de la synthèse',
  'alerts.overview.synthesis.challenge.supported':
    'Développez chaque signal lié ci-dessous et comparez leur chronologie avant de retenir cette cause.',
  'alerts.overview.synthesis.challenge.observationSet':
    "Ces observations sont liées, mais Pulse n'a pas établi de cause racine.",
  'alerts.overview.acknowledgedBadge': 'Acquittée',
  'alerts.overview.empty.description':
    'Les alertes apparaîtront ici lorsque des seuils seront dépassés',
  'alerts.overview.empty.title': 'Aucune alerte active',
  'alerts.overview.filteredEmpty.all': 'Aucune alerte active',
  'alerts.overview.filteredEmpty.unacknowledged': 'Aucune alerte non acquittée',
  'alerts.overview.hydration.pending.description':
    "Pulse confirme l'état actuel des incidents avant d'annoncer que tout est normal.",
  'alerts.overview.hydration.pending.title': 'Vérification des alertes actives',
  'alerts.overview.hydration.retry': 'Réessayer',
  'alerts.overview.hydration.unavailable.description':
    "Pulse n'a pas pu confirmer l'état actuel des incidents. Cette page n'annoncera pas que tout est normal tant que les données d'alerte ne seront pas disponibles.",
  'alerts.overview.hydration.unavailable.title':
    "L'état des alertes actives est temporairement indisponible",
  'alerts.overview.nodePrefix': 'sur {node}',
  'alerts.overview.notification.acknowledged': 'Alerte acquittée',
  'alerts.overview.notification.acknowledgeFailed': "Échec de l'acquittement de l'alerte",
  'alerts.overview.notification.bulkFailure.singular': "Échec de l'acquittement de {count} alerte.",
  'alerts.overview.notification.bulkFailure.plural': "Échec de l'acquittement de {count} alertes.",
  'alerts.overview.notification.bulkFailureGeneric': "Échec de l'acquittement des alertes",
  'alerts.overview.notification.bulkSuccess.singular': '{count} alerte acquittée.',
  'alerts.overview.notification.bulkSuccess.plural': '{count} alertes acquittées.',
  'alerts.overview.notification.restored': 'Alerte rétablie',
  'alerts.overview.notification.restoreFailed': "Échec du rétablissement de l'alerte",
  'alerts.overview.notification.resumed': "Notifications de l'alerte reprises",
  'alerts.overview.notification.resumeFailed': "Échec de la reprise des notifications de l'alerte",
  'alerts.overview.notification.snoozed': "Alerte reportée jusqu'à {until}",
  'alerts.overview.notification.snoozeFailed': "Échec du report de l'alerte",
  'alerts.overview.paused.description':
    'Réactivez les alertes pour reprendre la supervision et débloquer les onglets de configuration',
  'alerts.overview.paused.title': 'Les alertes sont en pause',
  'alerts.overview.section.activeAlerts': 'Alertes actives',
  'alerts.overview.summary.warning': 'Avertissement : {count}',
  'alerts.overview.summary.critical': 'Critique : {count}',
  'alerts.delivery.paused.title': 'Les notifications sont en pause',
  'alerts.delivery.paused.action': "Activer l'envoi",
  'alerts.delivery.paused.setupLink': 'Configurer la destination des alertes',
  'alerts.delivery.paused.reason.detectionOff':
    'Les alertes sont désactivées dans la configuration des alertes.',
  'alerts.delivery.paused.reason.snoozed': "L'envoi des notifications est reporté.",
  'alerts.delivery.paused.reason.notActivated': "L'envoi des notifications est désactivé.",
  'alerts.delivery.paused.consequence.destinations':
    'Pulse continue de détecter les alertes, mais aucune ne sera envoyée aux destinations ci-dessous. Les messages de test contournent la pause : un test réussi ne signifie donc pas que les alertes réelles sont transmises.',
  'alerts.delivery.paused.consequence.destinationsDetectionOff':
    "Pulse ne déclenche pas de nouvelles alertes tant qu'elles sont désactivées, et aucune ne sera envoyée aux destinations ci-dessous. Les messages de test contournent la pause : un test réussi ne signifie donc pas que les alertes réelles sont transmises.",
  'alerts.delivery.paused.consequence.overview':
    "Pulse continue de détecter les alertes ci-dessous, mais personne n'en est averti.",
  'alerts.delivery.paused.consequence.overviewDetectionOff':
    "Pulse ne déclenche pas de nouvelles alertes tant qu'elles sont désactivées, donc personne n'est averti.",
  'alerts.overview.snooze.description':
    "Suspend les notifications et l'escalade pour cet incident. La supervision continue et la résolution est toujours détectée.",
  'alerts.overview.snooze.option.eightHours': 'Pendant 8 heures',
  'alerts.overview.snooze.option.oneHour': 'Pendant 1 heure',
  'alerts.overview.snooze.option.sevenDays': 'Pendant 7 jours',
  'alerts.overview.snooze.option.tomorrowMorning': "Jusqu'à demain 9:00",
  'alerts.overview.snooze.option.twoHours': 'Pendant 2 heures',
  'alerts.overview.snooze.title': 'Reporter cette alerte',
  'alerts.overview.snoozedUntil': "Reportée jusqu'à {until}",
  'alerts.overview.startedAt': 'Début : {startedAt}',
  'alerts.overview.stats.acknowledged': 'Acquittées',
  'alerts.overview.stats.triggered24h': 'Déclenchées (24 h)',
  'alerts.overview.stats.workloadOverrides': 'Exceptions de charges de travail',
  'alerts.page.default.description':
    "Consultez les incidents actifs, examinez l'historique des alertes et gérez les seuils, les notifications et les plannings.",
  'alerts.page.default.title': 'Alertes',
  'alerts.page.destinations.description':
    "Acheminez les notifications d'alerte vers l'e-mail, Apprise, des webhooks et des notifications push mobiles.",
  'alerts.page.destinations.title': 'Notifications',
  'alerts.page.history.description':
    'Recherchez les alertes passées, consultez la chronologie des incidents et examinez la fréquence des alertes dans le temps.',
  'alerts.page.history.title': 'Historique des alertes',
  'alerts.page.overview.description':
    'Consultez les incidents actifs et vérifiez la couverture actuelle des alertes sur les ressources supervisées.',
  'alerts.page.overview.title': 'Aperçu des alertes',
  'alerts.page.schedule.description':
    "Définissez les heures silencieuses, le regroupement, les délais de répit, la récupération et l'escalade pour l'envoi des alertes.",
  'alerts.page.schedule.title': "Calendrier d'envoi",
  'alerts.page.thresholds.description':
    "Ajustez les seuils et les exceptions ciblées pour l'infrastructure, les charges de travail et les intégrations.",
  'alerts.page.thresholds.title': "Seuils d'alerte",
  'alerts.tabs.disabledTitle': 'Activez les alertes pour configurer ce paramètre',
  'alerts.tabs.group.configuration': 'Configuration',
  'alerts.tabs.group.status': 'État',
  'alerts.tabs.destinations': 'Notifications',
  'alerts.tabs.history': 'Historique',
  'alerts.tabs.overview': "Vue d'ensemble",
  'alerts.tabs.schedule': 'Planning',
  'alerts.tabs.thresholds': 'Seuils',
  'alerts.timeline.acknowledged': 'acquittée',
  'alerts.timeline.closedAt': 'clos {closedAt}',
  'alerts.timeline.empty': 'Aucun événement dans la chronologie pour le moment.',
  'alerts.timeline.failure': 'Échec du chargement de la chronologie.',
  'alerts.timeline.filterEmpty': 'Aucun événement ne correspond aux filtres sélectionnés.',
  'alerts.timeline.filterLabel.compact': 'Filtres',
  'alerts.timeline.filterLabel.panel': 'Filtrer les événements :',
  'alerts.timeline.heading': 'Incident',
  'alerts.timeline.loading': 'Chargement de la chronologie...',
  'alerts.timeline.event.alertAcknowledged': 'Acquittée',
  'alerts.timeline.event.alertFired': 'Déclenchée',
  'alerts.timeline.event.alertResolved': 'Résolue',
  'alerts.timeline.event.alertUnacknowledged': 'Non acquittée',
  'alerts.timeline.event.alertSnoozed': 'Reportée',
  'alerts.timeline.event.alertUnsnoozed': 'Reprise',
  'alerts.timeline.event.aiAnalysis': 'Patrol',
  'alerts.timeline.event.command': 'Commande',
  'alerts.timeline.event.note': 'Remarque',
  'alerts.timeline.event.runbook': 'Procédure',
  'alerts.timeline.noteLabel': "Remarque d'incident",
  'alerts.timeline.notePlaceholder': 'Ajouter une remarque pour cet incident...',
  'alerts.timeline.openedAt': 'ouvert {openedAt}',
  'alerts.timeline.quickFilter.all': 'Tous',
  'alerts.timeline.quickFilter.none': 'Aucun',
  'alerts.timeline.retry': 'Réessayer',
  'alerts.timeline.saveNote': 'Enregistrer la remarque',
  'alerts.timeline.savingNote': 'Enregistrement...',
  'alerts.timeline.unavailable': "Aucune chronologie d'incident disponible.",
  'runtimeHome.openingWorkspace': "Ouverture de l'espace de travail...",
  'setup.completion.action.addInfrastructure': 'Ajouter une infrastructure',
  'setup.completion.action.copyAdminToken': "Copier le jeton API d'administration",
  'setup.completion.action.copyPassword': 'Copier le mot de passe',
  'setup.completion.action.downloadCredentials': 'Télécharger les identifiants',
  'setup.completion.action.installAgent': 'Installer Pulse Agent',
  'setup.completion.action.openInfrastructure': 'Ouvrir Infrastructure',
  'setup.completion.connectedSummary.singular': 'Connecté ({count} système)',
  'setup.completion.connectedSummary.plural': 'Connecté ({count} systèmes)',
  'setup.completion.credentials.apiTokenLabel': "Jeton API d'administration",
  'setup.completion.credentials.badge': "Affiché pendant l'installation",
  'setup.completion.credentials.continuation.empty': 'Ajouter une infrastructure.',
  'setup.completion.credentials.continuation.connected':
    'Infrastructure ou Ajouter une infrastructure.',
  'setup.completion.credentials.description':
    "Enregistrez l'identifiant administrateur et le jeton API avant de quitter cet écran, puis continuez vers {destination}",
  'setup.completion.credentials.passwordLabel': 'Mot de passe',
  'setup.completion.credentials.title': 'Identifiants à enregistrer maintenant',
  'setup.completion.credentials.usernameLabel': "Nom d'utilisateur",
  'setup.completion.download.content':
    "Identifiants Pulse\n==================\nGénéré le : {generatedAt}\n\nConnexion web :\n---------------\nURL: {baseUrl}\nNom d'utilisateur : {username}\nMot de passe : {password}\n\nJeton API d'administration :\n----------------------------\n{apiToken}\n\nInfrastructure :\n----------------\n{infrastructureUrl}\n\nUtilisez Ajouter une infrastructure pour choisir une API de plateforme, Pulse Agent, ou les deux,\npour le premier système que Pulse doit superviser.\n\nConservez ces identifiants en lieu sûr !\n",
  'setup.completion.hero.connected.description.agent':
    'Votre compte administrateur est prêt et Pulse reçoit déjà des données de télémétrie. Ouvrez Infrastructure pour examiner le premier système, puis revenez à Ajouter une infrastructure lorsque vous voudrez une autre source Pulse Agent ou API de plateforme.',
  'setup.completion.hero.connected.description.api':
    'Votre compte administrateur est prêt et Pulse reçoit déjà des données de télémétrie. Ouvrez Infrastructure pour examiner le premier système, puis revenez à Ajouter une infrastructure lorsque vous voudrez une autre source API de plateforme ou Pulse Agent.',
  'setup.completion.hero.connected.description.both':
    'Votre compte administrateur est prêt et Pulse reçoit déjà des données de télémétrie. Ouvrez Infrastructure pour examiner le premier système, puis revenez à Ajouter une infrastructure lorsque vous voudrez une autre source API de plateforme ou Agent.',
  'setup.completion.hero.connected.title': 'Premier système supervisé connecté',
  'setup.completion.hero.empty.description':
    "Votre compte administrateur est prêt. Indiquez maintenant à Pulse ce qu'il doit superviser : un serveur Proxmox, un NAS, VMware, ou n'importe quelle machine Linux, Windows ou macOS.",
  'setup.completion.hero.empty.title': 'Connectez votre premier système',
  'setup.completion.nextStep.ariaLabel': "Étape suivante de l'installation",
  'setup.completion.nextStep.badge': 'Étape suivante recommandée',
  'setup.completion.nextStep.detail.agent':
    "Utilisez Ajouter une infrastructure à tout moment pour ajouter d'autres machines, ou pour connecter une plateforme comme Proxmox via son API.",
  'setup.completion.nextStep.detail.api':
    "Utilisez Ajouter une infrastructure à tout moment pour connecter d'autres systèmes, ou pour installer Pulse Agent sur les nœuds Proxmox afin d'obtenir une télémétrie locale à l'hôte, comme les températures et les données SMART, si elles sont disponibles.",
  'setup.completion.nextStep.detail.both':
    "Utilisez Ajouter une infrastructure à tout moment pour connecter d'autres systèmes via leur API, avec l'agent, ou les deux.",
  'setup.completion.nextStep.detail.empty':
    "Connectez Proxmox, TrueNAS ou VMware via leur API, ou exécutez l'installateur de Pulse Agent sur un hôte autonome. Sur les nœuds Proxmox, l'agent peut ajouter les températures locales et les données SMART lorsqu'elles sont disponibles.",
  'setup.completion.nextStep.label': 'Étape suivante',
  'setup.completion.nextStep.summary.connected.singular':
    'Ouvrez Infrastructure pour examiner votre premier système connecté.',
  'setup.completion.nextStep.summary.connected.plural':
    'Ouvrez Infrastructure pour examiner vos systèmes connectés.',
  'setup.completion.nextStep.summary.empty':
    'Ouvrez Ajouter une infrastructure et choisissez une connexion API de plateforme ou un hôte sur lequel installer Pulse Agent.',
  'setup.completion.nextStep.title.connected': 'Ouvrir Infrastructure',
  'setup.completion.nextStep.title.empty': 'Ajouter le premier système à superviser',
  'setup.completion.proActivation.action': "Saisir la clé d'activation",
  'setup.completion.proActivation.description':
    "Ce serveur exécute la version Pulse Pro sans licence active. Saisissez la clé d'activation reçue dans votre e-mail d'achat pour débloquer les fonctionnalités Pro.",
  'setup.completion.proActivation.title': 'Activer Pulse Pro',
  'setup.completion.resource.unknownName': 'Inconnu',
  'setup.completion.sourceOptions.agent.description':
    "Exécutez l'installateur sur un hôte Linux, Windows ou macOS. Pulse Agent remonte la télémétrie de l'hôte. Les températures, SMART, Docker et Kubernetes ne sont disponibles que là où ils sont pris en charge.",
  'setup.completion.sourceOptions.agent.title': 'Pulse Agent',
  'setup.completion.sourceOptions.both.description':
    "Connectez Proxmox par API pour l'ensemble du cluster, puis ajoutez l'agent sur les nœuds où vous voulez une télémétrie locale à l'hôte, comme les températures et les données SMART, si elles sont disponibles.",
  'setup.completion.sourceOptions.both.title': 'Utiliser les deux',
  'setup.completion.sourceOptions.platformApi.description':
    "Proxmox VE, Proxmox Backup Server, Proxmox Mail Gateway, TrueNAS ou VMware. Pulse lit les VM, le stockage et l'état de santé via l'API de la plateforme.",
  'setup.completion.sourceOptions.platformApi.title': 'Connecter par API',
  'setup.completion.sourceOptions.title': 'Modes de connexion',
  'setup.progress.ariaLabel': "Progression de l'installation",
  'setup.progress.state.completed': ', terminée',
  'setup.progress.state.current': ', en cours',
  'setup.progress.stepAriaLabel': 'Étape {index} : {step}{state}',
  'setup.security.action.back': 'Retour',
  'setup.security.action.createAccount': 'Créer le compte et continuer',
  'setup.security.action.settingUp': 'Configuration...',
  'setup.security.customPassword': 'Mot de passe personnalisé',
  'setup.security.description':
    "L'étape suivante consiste à choisir la première source d'infrastructure.",
  'setup.security.error.passwordMismatch': 'Les mots de passe ne correspondent pas',
  'setup.security.error.passwordRequired': 'Veuillez saisir un mot de passe',
  'setup.security.error.passwordTooShort': 'Le mot de passe doit contenir au moins 12 caractères',
  'setup.security.error.setupFailed': "Échec de l'installation : {error}",
  'setup.security.error.telemetryLockedOn':
    "Votre compte administrateur a été créé. Les statistiques d'utilisation restent activées, car ce serveur définit PULSE_TELEMETRY=true. Définissez PULSE_TELEMETRY=false et redémarrez Pulse pour les désactiver.",
  'setup.security.error.telemetryDisableFailed':
    "Votre compte administrateur a été créé, mais les statistiques d'utilisation n'ont pas pu être désactivées. Vous pouvez les désactiver dans Paramètres → Système → Général.",
  'setup.security.generatedPasswordHelp':
    "Un mot de passe sécurisé de 20 caractères sera généré et affiché sur l'écran suivant.",
  'setup.security.label.confirmPassword': 'Confirmer le mot de passe',
  'setup.security.label.password': 'Mot de passe',
  'setup.security.label.username': "Nom d'utilisateur",
  'setup.security.minimumPasswordHelp': '12 caractères minimum.',
  'setup.security.nextScreen.itemApiToken': "Un jeton API d'administration pour l'automatisation",
  'setup.security.nextScreen.itemCredentials': "Votre nom d'utilisateur et votre mot de passe",
  'setup.security.nextScreen.saveOnce':
    "Enregistrez-les avant de continuer, ils ne s'affichent qu'une seule fois.",
  'setup.security.nextScreen.title': "Sur l'écran suivant",
  'setup.security.passwordMode.autoGenerate': 'Générer automatiquement',
  'setup.security.placeholder.password': 'Mot de passe (12 caractères min.)',
  'setup.security.placeholder.username': 'admin',
  'setup.security.showPassword.hide': 'Masquer',
  'setup.security.showPassword.show': 'Afficher',
  'setup.security.telemetry.description':
    "Envoyez un petit résumé quotidien d'utilisation pour que les fonctionnalités et plateformes dont vous dépendez soient traitées en priorité. Il contient des compteurs et des indicateurs activé/désactivé avec un identifiant d'installation pseudonyme renouvelé régulièrement, jamais de noms d'hôte, d'identifiants ni d'adresses IP. Il n'est jamais vendu ni partagé et n'est lié à aucun compte Pulse. Modifiable à tout moment dans les Paramètres.",
  'setup.security.telemetry.title': "Statistiques d'utilisation",
  'setup.security.title': 'Créez votre compte administrateur',
  'setup.step.firstSource': 'Première source',
  'setup.step.security': 'Sécurité',
  'setup.step.unlockServer': 'Déverrouiller le serveur',
  'setup.welcome.telemetryNotice.description':
    "Pulse envoie un petit résumé quotidien d'utilisation indiquant quelles versions sont déployées et quelles fonctionnalités et plateformes sont utilisées, afin d'orienter le développement là où Pulse sert réellement. Il contient des compteurs et des indicateurs activé/désactivé avec un identifiant d'installation pseudonyme renouvelé régulièrement, jamais de noms d'hôte, d'identifiants, d'adresses IP ni rien de ce que vous saisissez. Il n'est jamais vendu, partagé ni utilisé à des fins publicitaires, et n'est lié à aucun compte ni licence Pulse. Il est activé par défaut. Vous ferez votre choix à l'étape suivante et pourrez le modifier à tout moment dans les Paramètres.",
  'setup.welcome.telemetryNotice.title': "Statistiques d'utilisation",
  'setup.wizard.ariaLabel': "Assistant d'installation de Pulse",
  'setup.welcome.action.continueSecurity': 'Continuer vers Sécurité',
  'setup.welcome.action.verifyToken': "Vérifier le jeton d'amorçage",
  'setup.welcome.action.verifyingToken': "Vérification du jeton d'amorçage...",
  'setup.welcome.copyCommandTitle': 'Copier la commande',
  'setup.welcome.deploymentHint.choose':
    "Exécutez la commande correspondant à la façon dont Pulse est installé. Le serveur ne révèle aucun détail de déploiement avant le déverrouillage de l'installation.",
  'setup.welcome.deploymentHint.containerized':
    "Pulse semble s'exécuter dans un environnement conteneurisé. Exécutez la commande depuis l'hôte qui gère le conteneur pour afficher le jeton d'installation à usage unique.",
  'setup.welcome.deploymentHint.direct':
    "Exécutez la commande directement dans un shell sur le serveur Pulse pour afficher le jeton d'installation à usage unique.",
  'setup.welcome.deploymentHint.dockerNamed':
    "Pulse semble s'exécuter dans Docker sous le nom de conteneur \"{containerName}\". Exécutez la commande sur l'hôte Docker pour afficher le jeton d'installation à usage unique depuis ce conteneur.",
  'setup.welcome.deploymentHint.dockerUnnamed':
    "Pulse semble s'exécuter dans Docker. Exécutez la commande sur l'hôte Docker en remplaçant <pulse-container> par le nom du conteneur Pulse en cours d'exécution.",
  'setup.welcome.deploymentHint.lxc':
    "Pulse semble s'exécuter dans le conteneur LXC {ctid}. Exécutez la commande sur l'hôte Proxmox pour entrer dans ce conteneur et afficher le jeton d'installation à usage unique.",
  'setup.welcome.deploymentLabel.containerConsole':
    'Console du conteneur (Unraid, Portainer, applications TrueNAS)',
  'setup.welcome.deploymentLabel.containerized': 'Déploiement conteneurisé',
  'setup.welcome.deploymentLabel.direct': "Installation directe sur l'hôte",
  'setup.welcome.deploymentLabel.docker': 'Déploiement Docker',
  'setup.welcome.deploymentLabel.lxc': 'Conteneur LXC',
  'setup.welcome.error.copyCommandFailed': 'Échec de la copie de la commande',
  'setup.welcome.error.invalidBootstrapToken':
    "Jeton d'amorçage invalide. Veuillez vérifier et réessayer.",
  'setup.welcome.error.invalidBootstrapTokenResponse': "Jeton d'amorçage invalide",
  'setup.welcome.error.missingBootstrapToken': "Veuillez saisir le jeton d'amorçage",
  'setup.welcome.error.snapshotPaste':
    "Ceci ressemble au contenu chiffré du fichier .bootstrap_token, et non au jeton d'installation brut. Exécutez la commande correspondante ci-dessus et collez la chaîne du jeton qu'elle affiche.",
  'setup.welcome.hero.coverage':
    'Connectez une API de plateforme, installez Pulse Agent, ou utilisez les deux pour une couverture complète.',
  'setup.welcome.hero.step.admin': 'Créer le compte administrateur',
  'setup.welcome.hero.step.source': 'Choisir la première source',
  'setup.welcome.hero.step.unlock': 'Déverrouiller ce serveur Pulse',
  'setup.welcome.hero.stepsIntro': 'Trois étapes :',
  'setup.welcome.hero.stepsThen': 'puis',
  'setup.welcome.hero.title': 'Bienvenue dans Pulse',
  'setup.welcome.placeholder.bootstrapToken': "Collez votre jeton d'amorçage",
  'setup.welcome.success.commandCopied': 'Commande copiée dans le presse-papiers',
  'setup.welcome.telemetryNotice.detailsLink': 'Tous les détails',
  'setup.welcome.tokenHelp.afterVerify':
    "Une fois ce jeton vérifié par Pulse, l'étape suivante consiste à créer le compte administrateur de ce serveur.",
  'setup.welcome.tokenHelp.docker':
    "Ce jeton d'amorçage à usage unique sert uniquement à déverrouiller la première installation. Exécutez la commande ci-dessus et collez la chaîne du jeton qu'elle affiche. Après vérification, vous créerez le compte administrateur et Pulse générera séparément le jeton API de longue durée.",
  'setup.welcome.tokenHelp.host':
    "Ce jeton d'amorçage à usage unique sert uniquement à déverrouiller la première installation sur ce serveur Pulse. Exécutez la commande ci-dessus et collez la chaîne du jeton qu'elle affiche. Ce n'est ni votre mot de passe administrateur, ni le jeton API que vous utiliserez après l'installation.",
  'setup.welcome.tokenHelp.generic':
    "Ce jeton d'amorçage à usage unique sert uniquement à déverrouiller la première installation sur ce serveur Pulse. Exécutez la commande correspondante ci-dessus et collez la chaîne du jeton qu'elle affiche. Ce n'est ni votre mot de passe administrateur, ni le jeton API de longue durée.",
  'setup.welcome.tokenHelp.title': 'À quoi sert ce jeton',
  'setup.welcome.unlockTitle': "Déverrouiller l'installation",
  'settings.general.appearance.title': 'Apparence',
  'settings.general.fullWidth.description':
    'Étendre le contenu à toute la largeur disponible sur les grands écrans.',
  'settings.general.fullWidth.title': 'Mode pleine largeur',
  'settings.general.docker.envHint': "Peut aussi être défini via la variable d'environnement :",
  'settings.general.docker.section.description':
    "Contrôlez l'affichage des actions de mise à jour {sourceLabel} dans Pulse.",
  'settings.general.docker.section.title': 'Mises à jour {sourceLabel}',
  'settings.general.docker.toggle.description':
    'Lorsque cette option est activée, les actions "Update" de {sourceLabel} sont masquées dans Pulse. La détection des mises à jour continue de fonctionner, les mises à jour disponibles restent donc visibles.',
  'settings.general.docker.toggle.title': 'Masquer les boutons de mise à jour',
  'settings.general.guestDocker.section.title': 'Docker dans les LXC Proxmox',
  'settings.general.guestDocker.section.description':
    'Découvrez les conteneurs Docker qui tournent dans les invités LXC Proxmox sans installer un agent dans chaque invité.',
  'settings.general.guestDocker.toggle.title': 'Découvrir Docker dans les invités LXC',
  'settings.general.guestDocker.toggle.description':
    "Lorsque cette option est activée, Pulse lance des vérifications pct exec limitées via les agents des nœuds Proxmox connectés pour trouver Docker dans les invités LXC et lister leurs conteneurs. Les invités qui remontent déjà leurs données via leur propre agent sont ignorés, et Pulse ne collecte que la liste des conteneurs et leurs statistiques d'utilisation — aucun détail d'inspection, d'environnement, de montage, de commande ou de processus.",
  'settings.general.guestDocker.requirements':
    "Nécessite un agent Pulse sur le nœud Proxmox avec l'exécution de commandes activée. Administrateurs uniquement.",
  'settings.general.language.ariaLabel': "Langue de l'application",
  'settings.general.language.description':
    "Utiliser cette langue pour l'interface de l'application. Les commandes, noms de ressources et champs d'API restent inchangés.",
  'settings.general.language.title': 'Langue',
  'settings.general.monitoringCadence.current':
    'Fréquence actuelle : {seconds} secondes ({duration})',
  'settings.general.monitoringCadence.custom.description':
    "Saisissez une valeur en secondes ({min}-{max}). S'applique à tous les clusters.",
  'settings.general.monitoringCadence.custom.title': "Intervalle d'interrogation personnalisé",
  'settings.general.monitoringCadence.description':
    "Des intervalles plus courts offrent des mises à jour quasi temps réel, au prix d'une utilisation API et CPU plus élevée sur chaque nœud. Choisissez un intervalle plus long pour réduire la charge sur les clusters très sollicités.",
  'settings.general.monitoringCadence.duration.minute': '{count} minute',
  'settings.general.monitoringCadence.duration.minutes': '{count} minutes',
  'settings.general.monitoringCadence.duration.underMinute': "moins d'une minute",
  'settings.general.monitoringCadence.envLocked': "Géré par la variable d'environnement {envVar}.",
  'settings.general.monitoringCadence.preset.balanced': 'Équilibrée ({duration})',
  'settings.general.monitoringCadence.preset.custom': 'Personnalisée',
  'settings.general.monitoringCadence.preset.low': 'Faible ({duration})',
  'settings.general.monitoringCadence.preset.realtime': 'Temps réel ({duration})',
  'settings.general.monitoringCadence.preset.veryLow': 'Très faible ({duration})',
  'settings.general.monitoringCadence.section.description':
    'Contrôlez la fréquence à laquelle Pulse interroge les nœuds Proxmox VE.',
  'settings.general.monitoringCadence.section.title': 'Fréquence de supervision',
  'settings.general.telemetry.description':
    "Aidez à améliorer Pulse en partageant un résumé quotidien d'utilisation. L'aperçu de la charge utile montre exactement ce qui serait envoyé : un identifiant d'installation pseudonyme renouvelé régulièrement, l'identité normalisée de la version, la plateforme d'exécution, des tranches grossières de déploiement et de cycle de vie, des nombres agrégés de ressources et de résultats, des indicateurs de fonctionnalités grossiers, des compteurs d'utilisation de Patrol, de l'Assistant et de l'API de capacités sans aucun contenu, et quatre compteurs d'adoption de l'historique des charges de travail dédupliqués par session. Il n'inclut jamais de noms d'hôte, d'identifiants, d'identifiants d'infrastructure, d'URL, de chemins, de paramètres régionaux, d'événements bruts du navigateur, de parcours de clics au niveau des événements, de prompts, de messages de chat, de texte de commandes, de résultats d'actions, de valeurs de jetons, de noms, d'adresses e-mail ni d'adresses IP. Il n'est jamais vendu, partagé avec qui que ce soit ni utilisé à des fins publicitaires, et n'est lié à aucun compte ni licence Pulse. Les lignes de télémétrie sont conservées jusqu'à 90 jours, et les adresses IP des requêtes ne sont utilisées que de manière transitoire pour la limitation de débit et ne sont pas stockées dans les lignes de télémétrie.",
  'settings.general.temperature.description':
    'Afficher les températures en degrés Celsius ou Fahrenheit.',
  'settings.general.temperature.title': 'Unité de température',
  'settings.general.telemetry.copyJson': 'Copier le JSON',
  'settings.general.telemetry.disabledPreview':
    "La télémétrie est actuellement désactivée. Cet aperçu montre la charge utile que Pulse enverrait si vous l'activiez.",
  'settings.general.telemetry.fullDetails': 'Tous les détails',
  'settings.general.telemetry.payloadAriaLabel': 'Aperçu de la charge utile de télémétrie',
  'settings.general.telemetry.payloadTitle': 'Charge utile actuelle du signal de vie',
  'settings.general.telemetry.previewPayload': 'Aperçu de la charge utile',
  'settings.general.telemetry.refreshPayload': 'Actualiser la charge utile',
  'settings.general.telemetry.resetId': "Réinitialiser l'identifiant",
  'settings.general.telemetry.section.description':
    "Contrôlez la télémétrie d'utilisation sortante de cette instance Pulse.",
  'settings.general.telemetry.section.title': "Données d'utilisation et confidentialité",
  'settings.general.telemetry.title': "Télémétrie d'utilisation sortante",
  'settings.general.theme.description':
    'Choisissez clair, sombre, ou synchronisez avec le thème de votre système.',
  'settings.general.theme.option.dark': 'Sombre',
  'settings.general.theme.option.light': 'Clair',
  'settings.general.theme.option.system': 'Système',
  'settings.general.theme.title': 'Thème',
  'settings.header.api.description':
    "Générez et gérez des jetons Pulse à portée limitée pour les agents, l'automatisation et les intégrations externes.",
  'settings.header.api.title': 'Accès API',
  'settings.header.infrastructure.description':
    "Ajoutez, découvrez et vérifiez l'infrastructure supervisée par Pulse.",
  'settings.header.infrastructure.readOnlyDescription':
    "Consultez les systèmes supervisés de premier niveau et l'état actuel de leurs remontées. Les modifications de configuration restent indisponibles dans cette session en lecture seule.",
  'settings.header.infrastructure.title': 'Infrastructure',
  'settings.header.monitoringAvailability.description':
    'Supervisez les équipements et services accessibles uniquement par leur point de terminaison, avec des sondes ping, TCP et HTTP.',
  'settings.header.monitoringAvailability.title': 'Contrôles de disponibilité',
  'settings.header.organizationAccess.description':
    "Gérez les invitations de l'organisation, les rôles des membres et les transferts de propriété.",
  'settings.header.organizationAccess.title': "Accès à l'organisation",
  'settings.header.organizationBilling.description':
    "Consultez l'offre de votre organisation, les politiques d'utilisation applicables et l'état de l'abonnement pour l'accès payant.",
  'settings.header.organizationBilling.title': 'Facturation et utilisation',
  'settings.header.organizationBillingAdmin.description':
    "Consultez et gérez l'état de facturation des locataires pour toutes les organisations (mode hébergé uniquement).",
  'settings.header.organizationBillingAdmin.title': 'Administration de la facturation',
  'settings.header.organizationOverview.description':
    "Consultez les métadonnées de l'organisation, l'étendue de ses membres et sa propriété.",
  'settings.header.organizationOverview.title': "Vue d'ensemble de l'organisation",
  'settings.header.organizationSharing.description':
    'Partagez des ressources entre organisations avec un accès à portée limitée.',
  'settings.header.organizationSharing.title': 'Partage entre organisations',
  'settings.header.securityAudit.description':
    'Consultez les événements de sécurité, les tentatives de connexion et les modifications de configuration.',
  'settings.header.securityAudit.title': "Journal d'audit",
  'settings.header.securityAuth.description':
    "Gérez l'authentification par mot de passe et la rotation des identifiants.",
  'settings.header.securityAuth.title': 'Authentification',
  'settings.header.securityDataHandling.description':
    'Voyez quels détails des ressources supervisées peuvent être résumés, doivent rester locaux ou sont masqués.',
  'settings.header.securityDataHandling.title': 'Confidentialité des ressources',
  'settings.header.securityOverview.description':
    "Visualisez votre posture de sécurité en un coup d'œil et surveillez l'état de l'authentification.",
  'settings.header.securityOverview.title': "Vue d'ensemble de la sécurité",
  'settings.header.securityRoles.description':
    'Définissez des rôles personnalisés et gérez des autorisations précises pour les utilisateurs et les jetons.',
  'settings.header.securityRoles.title': 'Rôles',
  'settings.header.securitySso.description': "Configurez les fournisseurs d'identité OIDC et SAML.",
  'settings.header.securitySso.title': "Fournisseurs d'authentification unique (SSO)",
  'settings.header.securityUsers.description':
    'Attribuez des rôles aux utilisateurs et consultez les autorisations effectives sur votre infrastructure.',
  'settings.header.securityUsers.title': 'Accès des utilisateurs',
  'settings.header.securityWebhooks.description':
    "Configurez l'envoi en temps réel des événements d'audit vers des systèmes externes.",
  'settings.header.securityWebhooks.title': "Webhooks d'audit",
  'settings.header.systemAi.description':
    "Configurez les fournisseurs, les modèles par défaut, l'état des fournisseurs, le budget et l'utilisation de Pulse Intelligence.",
  'settings.header.systemAi.title': 'Fournisseurs et modèles',
  'settings.header.systemAiAgents.description':
    "Connectez Claude Code, Codex et d'autres clients MCP aux outils contrôlés de Pulse.",
  'settings.header.systemAiAgents.title': 'Agents externes',
  'settings.header.systemAiPatrol.description':
    "Définissez quand Patrol s'exécute, ce qui le déclenche et quel modèle il utilise.",
  'settings.header.systemAiPatrol.title': 'Patrol',
  'settings.header.systemBilling.description': 'Offre, licence et mode Patrol de cette instance.',
  'settings.header.systemBilling.title': 'Offres et facturation',
  'settings.header.systemGeneral.description':
    "Gérez l'apparence, la mise en page et la fréquence de supervision par défaut.",
  'settings.header.systemGeneral.title': 'Général',
  'settings.header.systemNetwork.description':
    "Configurez l'URL publique, le CORS, l'intégration dans d'autres pages et les limites réseau des webhooks.",
  'settings.header.systemNetwork.title': 'Réseau',
  'settings.header.systemRecovery.description':
    "Gérez l'interrogation des sauvegardes et instantanés, ainsi que l'export et l'import de la configuration.",
  'settings.header.systemRecovery.title': 'Récupération',
  'settings.header.systemRelay.description':
    "Connectez des appareils Pulse Mobile appairés à cette instance. Pulse Mobile sera retiré le 31 mars 2027, et les téléphones appairés continueront de fonctionner jusqu'à cette date.",
  'settings.header.systemRelay.title': 'Pulse Mobile',
  'settings.header.systemUpdates.description':
    'Recherchez les nouvelles versions de Pulse, choisissez un canal de mise à jour et gérez les mises à jour automatiques. Les mises à jour des agents restent dans Infrastructure.',
  'settings.header.systemUpdates.title': 'Mises à jour',
  'settings.header.supportDiagnostics.description':
    'Lancez des contrôles de santé, validez la connectivité et exportez des instantanés de dépannage.',
  'settings.header.supportDiagnostics.title': 'Diagnostics et santé',
  'settings.header.supportLogs.description':
    'Consultez le flux de journaux Pulse en direct et téléchargez le tampon capturé pour le support.',
  'settings.header.supportLogs.title': 'Journaux système',
  'settings.header.supportReporting.description':
    "Exportez les données d'inventaire et générez des rapports de performance depuis l'interface des paramètres.",
  'settings.header.supportReporting.title': 'Données et rapports',
  'settings.nav.group.infrastructure': 'Infrastructure',
  'settings.nav.group.monitoring': 'Supervision',
  'settings.nav.group.organization': 'Organisation',
  'settings.nav.group.pulseIntelligence': 'Pulse Intelligence',
  'settings.nav.group.security': 'Sécurité',
  'settings.nav.group.support': 'Assistance',
  'settings.nav.group.system': 'Système',
  'settings.nav.item.apiAccess': 'Accès API',
  'settings.nav.item.auditLog': "Journal d'audit",
  'settings.nav.item.auditWebhooks': "Webhooks d'audit",
  'settings.nav.item.authentication': 'Authentification',
  'settings.nav.item.availabilityChecks': 'Contrôles de disponibilité',
  'settings.nav.item.billing': 'Facturation',
  'settings.nav.item.billingAdmin': 'Administration de la facturation',
  'settings.nav.item.dataReports': 'Données et rapports',
  'settings.nav.item.diagnosticsHealth': 'Diagnostics et santé',
  'settings.nav.item.externalAgents': 'Agents externes',
  'settings.nav.item.general': 'Général',
  'settings.nav.item.infrastructure': 'Infrastructure',
  'settings.nav.item.network': 'Réseau',
  'settings.nav.item.organizationAccess': 'Accès',
  'settings.nav.item.organizationOverview': "Vue d'ensemble",
  'settings.nav.item.patrol': 'Patrol',
  'settings.nav.item.plans': 'Offres et facturation',
  'settings.nav.item.providerModels': 'Fournisseurs et modèles',
  'settings.nav.item.recovery': 'Récupération',
  'settings.nav.item.remoteAccess': 'Pulse Mobile',
  'settings.nav.item.resourcePrivacy': 'Confidentialité des ressources',
  'settings.nav.item.roles': 'Rôles',
  'settings.nav.item.securityOverview': "Vue d'ensemble de la sécurité",
  'settings.nav.item.sharing': 'Partage',
  'settings.nav.item.singleSignOn': 'Authentification unique (SSO)',
  'settings.nav.item.systemLogs': 'Journaux système',
  'settings.nav.item.updates': 'Mises à jour',
  'settings.nav.item.users': 'Utilisateurs',
  'settings.shell.collapseSidebarLabel': 'Réduire la navigation des paramètres',
  'settings.shell.configurationLoading': 'Chargement de la configuration...',
  'settings.shell.discardLabel': 'Abandonner',
  'settings.shell.expandSidebarLabel': 'Développer la navigation des paramètres',
  'settings.shell.loading': 'Chargement des paramètres...',
  'settings.shell.mobileBackLabel': 'Paramètres',
  'settings.shell.mobileCloseLabel': 'Fermer la navigation des paramètres',
  'settings.shell.navigationAriaLabel': 'Navigation des paramètres',
  'settings.shell.navigationTitle': 'Paramètres',
  'settings.shell.saveChangesLabel': 'Enregistrer les modifications',
  'settings.shell.searchEmpty': 'Aucun paramètre trouvé pour "{query}"',
  'settings.shell.searchPlaceholder': 'Rechercher un paramètre...',
  'settings.shell.unsavedDescription':
    'Vos modifications seront perdues si vous quittez cette page.',
  'settings.shell.unsavedTitle': 'Modifications non enregistrées',
} as const satisfies Partial<I18nCatalog>;
