const axios = require('axios');

class NotificationService {
    constructor() {
        this.services = {
            discord: new DiscordService(),
            slack: new SlackService(),
            gotify: new GotifyService(),
            telegram: new TelegramService(),
            ntfy: new NtfyService(),
            teams: new TeamsService(),
            generic: new GenericWebhookService()
        };
    }

    detectService(webhookUrl) {
        if (!webhookUrl) return null;
        
        // Discord
        if (webhookUrl.includes('discord.com/api/webhooks') || 
            webhookUrl.includes('discordapp.com/api/webhooks')) {
            return 'discord';
        }
        
        // Slack
        if (webhookUrl.includes('slack.com/') || 
            webhookUrl.includes('hooks.slack.com')) {
            return 'slack';
        }
        
        // Gotify - check for /message?token= pattern
        if (webhookUrl.includes('/message?token=') || 
            webhookUrl.includes('/message') && webhookUrl.includes('token=')) {
            return 'gotify';
        }
        
        // Telegram Bot API
        if (webhookUrl.includes('api.telegram.org/bot')) {
            return 'telegram';
        }
        
        // ntfy.sh
        if (webhookUrl.includes('ntfy.sh') || 
            webhookUrl.includes('/publish') || 
            (webhookUrl.includes('ntfy') && !webhookUrl.includes('/'))) {
            return 'ntfy';
        }
        
        // Microsoft Teams
        if (webhookUrl.includes('webhook.office.com') || 
            webhookUrl.includes('outlook.office.com/webhook')) {
            return 'teams';
        }
        
        // Default to generic webhook
        return 'generic';
    }

    async send(webhookUrl, alert) {
        const serviceType = this.detectService(webhookUrl);
        const service = this.services[serviceType];
        
        if (!service) {
            throw new Error(`Unknown notification service type: ${serviceType}`);
        }
        
        console.log(`[NotificationService] Detected service type: ${serviceType} for URL: ${webhookUrl}`);
        return await service.send(webhookUrl, alert);
    }

    // Format alert priority for different services
    getPriority(alert) {
        if (alert.priority === 'critical' || alert.guest?.status === 'stopped') return 5;
        if (alert.priority === 'high') return 4;
        if (alert.priority === 'normal') return 3;
        return 2;
    }

    // Get a human-readable alert title
    getAlertTitle(alert) {
        if (alert.type === 'summary') {
            return `Multiple Alerts: ${alert.summary.total} triggered`;
        }
        return alert.rule?.name || 'Pulse Alert';
    }

    // Get alert message/description
    getAlertMessage(alert) {
        if (alert.type === 'summary') {
            return this.formatSummaryMessage(alert);
        }
        
        const parts = [];
        
        // Guest info
        if (alert.guest) {
            parts.push(`${alert.guest.type.toUpperCase()} ${alert.guest.vmid}: ${alert.guest.name}`);
            parts.push(`Node: ${alert.guest.node}`);
            if (alert.guest.status) {
                parts.push(`Status: ${alert.guest.status}`);
            }
        }
        
        // Metrics info
        if (alert.exceededMetrics && alert.exceededMetrics.length > 0) {
            parts.push('\nExceeded Thresholds:');
            alert.exceededMetrics.forEach(m => {
                const value = typeof m.currentValue === 'number' ? 
                    Math.round(m.currentValue) : m.currentValue;
                const threshold = typeof m.threshold === 'number' ? 
                    Math.round(m.threshold) : m.threshold;
                parts.push(`• ${m.metricType.toUpperCase()}: ${value}% (threshold: ${threshold}%)`);
            });
        } else if (alert.metric && alert.currentValue !== undefined) {
            parts.push(`\n${alert.metric.toUpperCase()}: ${alert.currentValue}`);
            if (alert.threshold !== undefined) {
                parts.push(`Threshold: ${alert.threshold}`);
            }
        }
        
        // Rule description
        if (alert.rule?.description) {
            parts.push(`\n${alert.rule.description}`);
        }
        
        return parts.join('\n');
    }

    formatSummaryMessage(alert) {
        const parts = [`${alert.summary.total} alerts triggered`];
        
        if (alert.summary.critical > 0) {
            parts.push(`Critical: ${alert.summary.critical}`);
        }
        
        // By type breakdown
        if (alert.summary.byType) {
            parts.push('\nBy Type:');
            Object.entries(alert.summary.byType).forEach(([type, count]) => {
                parts.push(`• ${type.toUpperCase()}: ${count}`);
            });
        }
        
        // By node breakdown
        if (alert.summary.byNode) {
            parts.push('\nBy Node:');
            Object.entries(alert.summary.byNode).forEach(([node, count]) => {
                parts.push(`• ${node}: ${count}`);
            });
        }
        
        return parts.join('\n');
    }
}

// Base service class
class BaseNotificationService {
    async send(webhookUrl, alert) {
        throw new Error('send() method must be implemented by subclass');
    }
    
    async post(url, data, headers = {}) {
        const defaultHeaders = {
            'Content-Type': 'application/json',
            'User-Agent': 'Pulse-Alert-System/1.0'
        };
        
        return await axios.post(url, data, {
            timeout: 10000,
            headers: { ...defaultHeaders, ...headers }
        });
    }
}

// Discord implementation
class DiscordService extends BaseNotificationService {
    async send(webhookUrl, alert) {
        const notificationService = new NotificationService();
        const color = this.getColor(alert);
        const fields = this.buildFields(alert);
        
        const payload = {
            embeds: [{
                title: `🚨 ${notificationService.getAlertTitle(alert)}`,
                description: notificationService.getAlertMessage(alert),
                color: color,
                fields: fields,
                footer: {
                    text: 'Pulse Alert System'
                },
                timestamp: new Date().toISOString()
            }]
        };
        
        return await this.post(webhookUrl, payload);
    }
    
    getColor(alert) {
        if (alert.type === 'summary') return 0xFF4500; // Orange
        if (alert.priority === 'critical') return 0xFF0000; // Red
        if (alert.priority === 'high') return 0xFFA500; // Orange
        return 0xFFFF00; // Yellow
    }
    
    buildFields(alert) {
        const fields = [];
        
        if (alert.type === 'summary') {
            fields.push(
                { name: 'Total Alerts', value: alert.summary.total.toString(), inline: true },
                { name: 'Critical', value: alert.summary.critical.toString(), inline: true }
            );
        } else if (alert.guest) {
            fields.push(
                { name: 'VM/Container', value: `${alert.guest.name} (${alert.guest.vmid})`, inline: true },
                { name: 'Node', value: alert.guest.node, inline: true },
                { name: 'Type', value: alert.guest.type.toUpperCase(), inline: true }
            );
        }
        
        return fields;
    }
}

// Slack implementation
class SlackService extends BaseNotificationService {
    async send(webhookUrl, alert) {
        const notificationService = new NotificationService();
        const color = this.getColor(alert);
        const fields = this.buildFields(alert);
        
        const payload = {
            text: `🚨 *${notificationService.getAlertTitle(alert)}*`,
            attachments: [{
                color: color,
                title: notificationService.getAlertTitle(alert),
                text: notificationService.getAlertMessage(alert),
                fields: fields,
                footer: 'Pulse Alert System',
                ts: Math.floor(Date.now() / 1000)
            }]
        };
        
        return await this.post(webhookUrl, payload);
    }
    
    getColor(alert) {
        if (alert.type === 'summary') return 'warning';
        if (alert.priority === 'critical') return 'danger';
        if (alert.priority === 'high') return 'warning';
        return '#FFFF00';
    }
    
    buildFields(alert) {
        const fields = [];
        
        if (alert.type === 'summary') {
            fields.push(
                { title: 'Total Alerts', value: alert.summary.total.toString(), short: true },
                { title: 'Critical', value: alert.summary.critical.toString(), short: true }
            );
        } else if (alert.guest) {
            fields.push(
                { title: 'VM/Container', value: `${alert.guest.name} (${alert.guest.vmid})`, short: true },
                { title: 'Node', value: alert.guest.node, short: true }
            );
        }
        
        return fields;
    }
}

// Gotify implementation
class GotifyService extends BaseNotificationService {
    async send(webhookUrl, alert) {
        const notificationService = new NotificationService();
        
        const payload = {
            title: notificationService.getAlertTitle(alert),
            message: notificationService.getAlertMessage(alert),
            priority: notificationService.getPriority(alert)
        };
        
        // Add extras for better formatting if needed
        if (alert.guest?.vmid || alert.type === 'summary') {
            payload.extras = {
                "client::display": {
                    "contentType": "text/plain"
                },
                "pulse::alert": {
                    "type": alert.type,
                    "guest": alert.guest?.name,
                    "node": alert.guest?.node,
                    "vmid": alert.guest?.vmid
                }
            };
        }
        
        return await this.post(webhookUrl, payload);
    }
}

// Telegram implementation
class TelegramService extends BaseNotificationService {
    async send(webhookUrl, alert) {
        const notificationService = new NotificationService();
        
        // Extract chat_id from URL if provided
        const urlParts = webhookUrl.match(/chat_id=([^&]+)/);
        const chatId = urlParts ? urlParts[1] : null;
        
        if (!chatId) {
            throw new Error('Telegram webhook URL must include chat_id parameter');
        }
        
        // Clean the URL to get base API endpoint
        const baseUrl = webhookUrl.split('?')[0];
        
        const payload = {
            chat_id: chatId,
            text: `🚨 *${notificationService.getAlertTitle(alert)}*\n\n${notificationService.getAlertMessage(alert)}`,
            parse_mode: 'Markdown',
            disable_notification: notificationService.getPriority(alert) < 4
        };
        
        return await this.post(baseUrl, payload);
    }
}

// ntfy.sh implementation
class NtfyService extends BaseNotificationService {
    async send(webhookUrl, alert) {
        const notificationService = new NotificationService();
        
        // Extract topic from URL
        let topic = 'pulse-alerts'; // default
        let baseUrl = webhookUrl;
        
        // Check if URL contains a topic
        const urlParts = webhookUrl.split('/');
        if (urlParts.length > 3 && urlParts[urlParts.length - 1]) {
            topic = urlParts[urlParts.length - 1];
            baseUrl = urlParts.slice(0, -1).join('/') + '/';
        }
        
        const priority = notificationService.getPriority(alert);
        const tags = [];
        
        // Add emoji tags based on priority
        if (priority >= 5) tags.push('rotating_light');
        else if (priority >= 4) tags.push('warning');
        else tags.push('information_source');
        
        // Add metric type as tag if available
        if (alert.metric) tags.push(alert.metric.toLowerCase());
        
        const payload = {
            topic: topic,
            title: notificationService.getAlertTitle(alert),
            message: notificationService.getAlertMessage(alert),
            priority: priority,
            tags: tags
        };
        
        // Add click action for guest details if available
        if (alert.guest?.vmid && alert.guest?.node) {
            payload.actions = [{
                action: "view",
                label: "View in Proxmox",
                url: `https://proxmox.local/#v1:0:=qemu/${alert.guest.vmid}`
            }];
        }
        
        return await this.post(baseUrl, payload);
    }
}

// Microsoft Teams implementation
class TeamsService extends BaseNotificationService {
    async send(webhookUrl, alert) {
        const notificationService = new NotificationService();
        
        // Teams uses Adaptive Cards format
        const card = {
            "@type": "MessageCard",
            "@context": "http://schema.org/extensions",
            "themeColor": this.getThemeColor(alert),
            "summary": notificationService.getAlertTitle(alert),
            "sections": [{
                "activityTitle": notificationService.getAlertTitle(alert),
                "activitySubtitle": alert.rule?.description || "Pulse Alert",
                "facts": this.buildFacts(alert),
                "markdown": true
            }]
        };
        
        // Add alert details section
        if (alert.type !== 'summary') {
            const detailsText = notificationService.getAlertMessage(alert);
            card.sections.push({
                "text": detailsText,
                "markdown": true
            });
        }
        
        return await this.post(webhookUrl, card);
    }
    
    getThemeColor(alert) {
        if (alert.type === 'summary') return "FF8C00"; // Orange
        if (alert.priority === 'critical') return "FF0000"; // Red
        if (alert.priority === 'high') return "FFA500"; // Orange
        return "FFFF00"; // Yellow
    }
    
    buildFacts(alert) {
        const facts = [];
        
        if (alert.type === 'summary') {
            facts.push(
                { name: "Total Alerts", value: alert.summary.total.toString() },
                { name: "Critical", value: alert.summary.critical.toString() }
            );
            
            // Add type breakdown
            for (const [type, count] of Object.entries(alert.summary.byType || {})) {
                if (count > 0) {
                    facts.push({ name: type.toUpperCase(), value: `${count} alert${count > 1 ? 's' : ''}` });
                }
            }
        } else if (alert.guest) {
            facts.push(
                { name: "VM/Container", value: `${alert.guest.name} (${alert.guest.vmid})` },
                { name: "Node", value: alert.guest.node },
                { name: "Type", value: alert.guest.type.toUpperCase() }
            );
            
            if (alert.metric && alert.currentValue !== undefined) {
                facts.push({ name: "Metric", value: `${alert.metric.toUpperCase()}: ${alert.currentValue}` });
            }
        }
        
        return facts;
    }
}

// Generic webhook implementation (fallback)
class GenericWebhookService extends BaseNotificationService {
    async send(webhookUrl, alert) {
        // Keep the existing generic format for compatibility
        const simplifiedAlert = {
            ...alert,
            isSingleMetric: alert.metricsCount === 1,
            isMultipleMetrics: alert.metricsCount > 1,
            primaryMetric: alert.exceededMetrics && alert.exceededMetrics.length > 0 
                ? alert.exceededMetrics[0].metricType 
                : alert.metric,
            alertType: alert.metricsCount === 1 && alert.exceededMetrics && alert.exceededMetrics[0]
                ? `${alert.exceededMetrics[0].metricType}_threshold` 
                : 'multiple_thresholds'
        };
        
        const payload = {
            timestamp: new Date().toISOString(),
            alert: simplifiedAlert
        };
        
        return await this.post(webhookUrl, payload);
    }
}

module.exports = NotificationService;