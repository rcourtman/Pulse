/**
 * Debounce Handler for Alert Notifications
 * Manages the completion of debounce periods and triggers delayed notifications
 */
class DebounceHandler {
    constructor(alertManager) {
        this.alertManager = alertManager;
        this.checkInterval = 5000; // Check every 5 seconds
        this.intervalId = null;
    }
    
    start() {
        if (this.intervalId) {
            return; // Already running
        }
        
        console.log('[DebounceHandler] Starting debounce completion checker');
        this.intervalId = setInterval(() => {
            this.checkDebounceCompletions();
        }, this.checkInterval);
        
        // Also check immediately
        this.checkDebounceCompletions();
    }
    
    stop() {
        if (this.intervalId) {
            clearInterval(this.intervalId);
            this.intervalId = null;
            console.log('[DebounceHandler] Stopped debounce completion checker');
        }
    }
    
    async checkDebounceCompletions() {
        const now = Date.now();
        let emailsToSend = [];
        let webhooksToSend = [];
        
        console.log(`[DebounceHandler] Checking debounce completions. Email cooldowns: ${this.alertManager.emailCooldowns.size}, Webhook cooldowns: ${this.alertManager.webhookCooldowns.size}`);
        
        // Check email debounces
        for (const [cooldownKey, cooldownInfo] of this.alertManager.emailCooldowns) {
            console.log(`[DebounceHandler] Checking email cooldown: ${cooldownKey}`, {
                debounceStarted: cooldownInfo.debounceStarted,
                lastSent: cooldownInfo.lastSent,
                debounceProcessed: cooldownInfo.debounceProcessed
            });
            
            if (cooldownInfo.debounceStarted && !cooldownInfo.lastSent && !cooldownInfo.debounceProcessed) {
                const debounceEnd = cooldownInfo.debounceStarted + (this.alertManager.emailCooldownConfig.debounceDelayMinutes * 60000);
                
                if (now >= debounceEnd) {
                    // Debounce period completed - find the alert
                    const alert = this.findAlertByCooldownKey(cooldownKey, 'email');
                    if (alert && alert.state === 'active' && !alert.emailSent) {
                        console.log(`[DebounceHandler] Email debounce completed for alert ${alert.id}`);
                        emailsToSend.push(alert);
                        // Mark as processed to prevent duplicate sending
                        cooldownInfo.debounceProcessed = true;
                    }
                }
            }
        }
        
        // Check webhook debounces
        for (const [cooldownKey, cooldownInfo] of this.alertManager.webhookCooldowns) {
            if (cooldownInfo.debounceStarted && !cooldownInfo.lastSent && cooldownInfo.debounceUntil) {
                if (now >= cooldownInfo.debounceUntil) {
                    // Debounce period completed - find the alert
                    const alert = this.findAlertByCooldownKey(cooldownKey, 'webhook');
                    if (alert && alert.state === 'active' && !alert.webhookSent) {
                        console.log(`[DebounceHandler] Webhook debounce completed for alert ${alert.id}`);
                        webhooksToSend.push(alert);
                    }
                }
            }
        }
        
        // Send emails
        if (emailsToSend.length > 0) {
            console.log(`[DebounceHandler] Sending ${emailsToSend.length} debounced emails`);
            
            // Always batch emails when multiple alerts are ready
            if (emailsToSend.length > 1 || this.alertManager.emailBatchEnabled) {
                console.log(`[DebounceHandler] Batching ${emailsToSend.length} emails`);
                // Add all to queue and trigger batch send
                emailsToSend.forEach(alert => {
                    this.alertManager.queueEmailNotification(alert);
                });
                // Force batch send now
                if (this.alertManager.sendBatchedEmails) {
                    await this.alertManager.sendBatchedEmails();
                }
            } else {
                // Send single email directly
                const alert = emailsToSend[0];
                try {
                    await this.alertManager.sendDirectEmailNotification(alert);
                    alert.emailSent = true;
                    
                    // Update cooldown to prevent resending
                    const cooldownKey = this.alertManager.getEmailCooldownKey(alert);
                    const cooldownInfo = this.alertManager.emailCooldowns.get(cooldownKey);
                    if (cooldownInfo) {
                        cooldownInfo.lastSent = now;
                        cooldownInfo.cooldownUntil = now + (this.alertManager.emailCooldownConfig.defaultCooldownMinutes * 60000);
                        delete cooldownInfo.debounceProcessed; // Clear the flag
                    }
                } catch (error) {
                    console.error(`[DebounceHandler] Failed to send debounced email for alert ${alert.id}:`, error);
                }
            }
        }
        
        // Send webhooks
        if (webhooksToSend.length > 0) {
            console.log(`[DebounceHandler] Sending ${webhooksToSend.length} debounced webhooks`);
            
            const webhookUrl = process.env.WEBHOOK_URL;
            if (webhookUrl) {
                // Use batcher for intelligent batching
                for (const alert of webhooksToSend) {
                    try {
                        await this.alertManager.webhookBatcher.queueAlert(alert, webhookUrl);
                        
                        // Update cooldown to prevent resending
                        const cooldownKey = this.alertManager.getWebhookCooldownKey(alert);
                        const cooldownInfo = this.alertManager.webhookCooldowns.get(cooldownKey);
                        if (cooldownInfo) {
                            cooldownInfo.lastSent = now;
                            cooldownInfo.cooldownUntil = now + (this.alertManager.webhookCooldownConfig.defaultCooldownMinutes * 60000);
                            delete cooldownInfo.debounceUntil; // Clear debounce flag
                        }
                    } catch (error) {
                        console.error(`[DebounceHandler] Failed to send debounced webhook for alert ${alert.id}:`, error);
                    }
                }
            }
        }
        
        // Save state if anything was sent
        if (emailsToSend.length > 0 || webhooksToSend.length > 0) {
            this.alertManager.saveActiveAlerts();
        }
    }
    
    findAlertByCooldownKey(cooldownKey, type) {
        // Parse cooldown key format: "ruleId-guestId-metric" or "ruleId-endpointId-node-vmid-metric"
        const parts = cooldownKey.split('-');
        
        // Search active alerts for matching alert
        for (const [alertKey, alert] of this.alertManager.activeAlerts) {
            if (alert.state !== 'active') continue;
            
            // For bundled alerts, the cooldown key might be different
            if (alert.rule.type === 'guest_bundled') {
                const expectedKey = `guest-bundled-${alert.guest.endpointId}-${alert.guest.node}-${alert.guest.vmid}-bundled`;
                if (cooldownKey === expectedKey) {
                    return alert;
                }
            } else {
                // Check if this alert matches the cooldown key
                const alertCooldownKey = type === 'email' ? 
                    this.alertManager.getEmailCooldownKey(alert) : 
                    this.alertManager.getWebhookCooldownKey(alert);
                    
                if (alertCooldownKey === cooldownKey) {
                    return alert;
                }
            }
        }
        
        return null;
    }
}

module.exports = DebounceHandler;