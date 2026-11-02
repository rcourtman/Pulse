# Bien démarrer avec Pulse en français

Cette page est le point d'entrée en français pour l'installation, la première
connexion et le choix d'une offre. La documentation complète et canonique reste
la documentation en anglais, dans [docs/README.md](../../README.md). Les
commandes, noms d'images, clés de configuration, clés d'activation et chemins de
l'interface sont volontairement conservés sans traduction.

## Qu'est-ce que Pulse ?

Pulse est un espace de supervision auto-hébergé pour Proxmox, Docker,
Kubernetes, TrueNAS et l'infrastructure associée. Community inclut gratuitement
la supervision de base. Pro ajoute l'analyse des causes racines, des workflows
de remédiation sécurisés, des outils d'exploitation, des fonctions de
gouvernance et 90 jours d'historique.

Relay n'est plus commercialisé. Les abonnés Relay existants bénéficient des
fonctions Pro à leur tarif actuel tant que leur abonnement se poursuit. Les
téléphones déjà appairés continuent de fonctionner jusqu'au
**31 mars 2027**. Relay connecte l'application mobile, pas l'interface web.
Pour accéder à l'interface web hors de chez vous, utilisez votre propre VPN ou
un tunnel. Pour recevoir les alertes sur votre téléphone, ajoutez une
destination ntfy, Gotify ou Pushover et ouvrez Pulse dans le navigateur de
votre téléphone.

## Clients payants Relay, Pro et legacy

Les assets des GitHub Releases et l'image Docker publique `rcourtman/pulse`
sont des builds Community. Activez votre licence dans
**Settings → Plans & Billing → License recovery** pour débloquer les fonctions Pro.

Ces builds Community n'incluent pas les hooks d'exécution privés de Pulse Pro
pour Audit Log, Audit Webhooks, RBAC et governed remediation. Pour cette
version, utilisez <https://pulserelay.pro/download.html> avec une
**v6 activation key**. Une v6 activation key commence par `ppk_live_`. Une clé
v5 ou legacy n'est pas une activation key `ppk_live_` et ne fonctionnera pas sur
cette page de téléchargement.

## Démarrage rapide : Proxmox LXC

Si vous utilisez Proxmox VE, l'installateur LXC officiel est la méthode la plus
native pour Pulse. Il installe le serveur Pulse. Remplacez `vX.Y.Z` par le tag
exact de la version à installer, vérifiez l'installateur signé et exécutez-le
sur votre hôte Proxmox :

```bash
export PULSE_VERSION=vX.Y.Z
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh" &&
curl -fsSLO "https://github.com/rcourtman/Pulse/releases/download/${PULSE_VERSION}/install.sh.sshsig" &&
ssh-keygen -Y verify \
  -f <(printf '%s\n' 'pulse-installer namespaces="pulse-install" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMZd/DaH+BldzOkq1A8KVTcFk73nAyrE8aJOyf7i00jm pulse-installer') \
  -I pulse-installer \
  -n pulse-install \
  -s install.sh.sshsig < install.sh &&
bash install.sh --version "${PULSE_VERSION}"
```

Collez le bloc en entier : si un téléchargement ou la vérification de la
signature échoue, l'installateur ne s'exécute pas. N'exécutez pas la dernière
commande seule pour contourner un échec.

Les installations d'agents et les mises à jour d'agents de v5 vers v6 utilisent
la commande générée par Pulse dans **Settings → Infrastructure → Install on a
host**. Cette commande est servie par votre serveur Pulse sur `/install.sh`.

## Démarrage rapide : Docker

Pour les environnements conteneurisés ou les tests, vous pouvez lancer Pulse
directement avec Docker :

```bash
docker run -d \
  --name pulse \
  -p 7655:7655 \
  -v pulse_data:/data \
  --restart unless-stopped \
  rcourtman/pulse:vX.Y.Z
```

Ouvrez ensuite Pulse sur `http://<your-ip>:7655`.

Avec Docker Compose, le même fichier peut servir aux images Community et aux
images Pro privées s'il utilise la variable `PULSE_IMAGE` :

```yaml
services:
  pulse:
    image: ${PULSE_IMAGE:-rcourtman/pulse:vX.Y.Z}
    container_name: pulse
    restart: unless-stopped
    ports:
      - "7655:7655"
    volumes:
      - pulse_data:/data

volumes:
  pulse_data:
```

Laissez `PULSE_AUTH_USER` et `PULSE_AUTH_PASS` non définis sur une nouvelle
installation et terminez la première connexion avec le jeton d'amorçage.
N'utilisez pas un mot de passe d'exemple partagé. Pour une configuration
automatisée, utilisez une source d'identifiants privée gérée par le
déploiement, voir
[Configuration](../../CONFIGURATION.md#private-docker-authentication-file).
Ne placez pas de mots de passe dans les arguments de commande ni dans un
fichier Compose partagé. Un administrateur Docker peut toujours lire les
identifiants dans l'environnement du conteneur, même si Pulse les hache pour
l'authentification.

## Première connexion

Au premier démarrage, vous devez récupérer un **Bootstrap Token** et l'utiliser
pour créer le compte administrateur.

| Plateforme | Commande |
|---|---|
| Docker | `docker exec pulse /app/pulse bootstrap-token` |
| Kubernetes | `kubectl exec -it <pod> -- /app/pulse bootstrap-token` |
| Systemd | `sudo pulse bootstrap-token` |

Collez la chaîne du jeton affichée par la commande. Ne collez pas directement
le contenu du fichier `.bootstrap_token`. En v6, ce fichier peut contenir un
instantané JSON chiffré au lieu du jeton utilisable pour l'installation.

1. Ouvrez `http://<your-ip>:7655`.
2. Collez le **Bootstrap Token**.
3. Terminez l'assistant **Quick Security Setup**.
4. Ouvrez **Settings → Infrastructure → Install on a host** si vous voulez
   installer l'unified agent sur des hôtes.

Pour Proxmox, commencez par une supervision API-only si l'inventaire, l'état
des nœuds, l'état des VM/conteneurs et les métriques de stockage vous
suffisent. Utilisez des agents pour la visibilité Docker/Podman dans les
invités, les données SMART/température de l'hôte, le détail local ZFS/Ceph/mdadm
ou toute autre télémétrie nécessitant un accès local à l'hôte.

### Proxmox : sécurité pendant les sauvegardes

**API-only ne signifie pas sans agent invité.** Les requêtes de systèmes de
fichiers et de mémoire des VM via QEMU Guest Agent peuvent partager le canal
utilisé par les sauvegardes avec gel des systèmes de fichiers. Des droits en
lecture seule ne prouvent pas que la sauvegarde est sûre. Pendant une
sauvegarde, un gel/dégel ou un incident avec un invité qui ne répond plus,
n'ajoutez pas de droits, n'installez ni ne redémarrez d'agents et n'envoyez pas
de requêtes manuelles à l'agent invité.

Pour les installations concernées, suivez la
[précaution de sécurité pour les sauvegardes](../../VM_DISK_MONITORING.md#backup-safety)
en anglais **avant une sauvegarde planifiée avec gel**. Elle tient compte du
serveur Pulse et des mises à jour automatiques, pas seulement de l'agent invité.
Ce n'est pas une procédure de reprise pour un incident en cours. Arrêter Pulse
n'annule pas les requêtes déjà envoyées à l'agent invité. **Tant que Pulse est
arrêté, la supervision et les alertes Pulse ne sont pas disponibles.** Prévoyez
une supervision indépendante.

Un résultat **OK** de la sauvegarde ne prouve pas que l'invité a été dégelé.
Laissez Pulse en pause jusqu'à ce que la sauvegarde soit terminée et que des
vérifications indépendantes **effectuées après sa fin**, sans Pulse ni QEMU
Guest Agent, confirment les trois points :

1. Les systèmes de fichiers ont été dégelés.
2. Des écritures récentes et réussies des charges de travail sur **chaque système de fichiers inclus dans la sauvegarde**.
3. Les charges de travail fonctionnent.

Si une vérification manque ou échoue, maintenez la pause et suivez la procédure
de reprise de la plateforme. Une connexion à la console ou une lecture réussie
ne suffit pas. Seulement après toutes les vérifications, ne redémarrez que les
services et minuteries qui **étaient actifs avant la pause**. Un état antérieur
inconnu n'autorise pas leur démarrage. Ne supprimez pas les verrous de
sauvegarde et ne désactivez pas le gel pour tester un problème de supervision.

## Étapes suivantes

- [Installation Guide](../../INSTALL.md) pour les méthodes d'installation complètes.
- [Configuration](../../CONFIGURATION.md) pour l'authentification, les
  notifications et les paramètres système.
- [Troubleshooting](../../TROUBLESHOOTING.md) pour les journaux et les problèmes
  courants.
- [Agent Security](../../AGENT_SECURITY.md) pour les privilèges des agents, les
  options Proxmox API-only et la vérification des signatures.
- [Plans and entitlements](../../PULSE_PRO.md) pour Community, Relay, Pro et
  Cloud.
