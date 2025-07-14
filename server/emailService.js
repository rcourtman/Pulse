const nodemailer = require('nodemailer');
const sgMail = require('@sendgrid/mail');

class EmailService {
    constructor() {
        this.provider = null;
        this.transporter = null;
        this.config = {};
    }

    /**
     * Initialize email service with configuration
     */
    async initialize(config) {
        // Determine provider based on config
        if (config.emailProvider === 'sendgrid' && config.sendgridApiKey) {
            this.provider = 'sendgrid';
            sgMail.setApiKey(config.sendgridApiKey);
            this.config = {
                from: config.sendgridFromEmail || config.from || config.alertFromEmail,
                to: config.alertToEmail || config.to
            };
        } else if (config.smtpHost || config.host) {
            this.provider = 'smtp';
            this.transporter = nodemailer.createTransport({
                host: config.smtpHost || config.host,
                port: parseInt(config.smtpPort || config.port) || 587,
                secure: config.smtpSecure || config.secure || false,
                requireTLS: true,
                auth: {
                    user: config.smtpUser || config.user,
                    pass: config.smtpPass || config.pass
                },
                tls: {
                    rejectUnauthorized: false
                }
            });
            this.config = {
                from: config.from || config.alertFromEmail,
                to: config.to || config.alertToEmail
            };
        } else {
            this.provider = null;
        }
    }

    /**
     * Send email using configured provider
     */
    async sendMail(mailOptions) {
        if (!this.provider) {
            throw new Error('Email service not configured');
        }

        if (this.provider === 'sendgrid') {
            const msg = {
                to: mailOptions.to,
                from: mailOptions.from || this.config.from,
                subject: mailOptions.subject,
                text: mailOptions.text,
                html: mailOptions.html
            };

            try {
                await sgMail.send(msg);
            } catch (error) {
                console.error('[EmailService] SendGrid error:', error);
                throw error;
            }
        } else if (this.provider === 'smtp' && this.transporter) {
            await this.transporter.sendMail(mailOptions);
        }
    }

    /**
     * Test email configuration
     */
    async testEmail(customConfig = null) {
        if (customConfig) {
            // Test with custom config
            if (customConfig.emailProvider === 'sendgrid' && customConfig.sendgridApiKey) {
                const testSgMail = require('@sendgrid/mail');
                testSgMail.setApiKey(customConfig.sendgridApiKey);
                
                const msg = {
                    to: customConfig.to,
                    from: customConfig.from,
                    subject: '🧪 Pulse Alert System - Test Email',
                    text: 'This is a test email from Pulse monitoring system.',
                    html: '<p>This is a test email from Pulse monitoring system.</p>'
                };

                await testSgMail.send(msg);
                return { success: true, provider: 'sendgrid' };
            } else {
                // Test SMTP
                const testTransporter = nodemailer.createTransport({
                    host: customConfig.host,
                    port: parseInt(customConfig.port) || 587,
                    secure: customConfig.secure || false,
                    requireTLS: true,
                    auth: {
                        user: customConfig.user,
                        pass: customConfig.pass || process.env.SMTP_PASS
                    },
                    tls: {
                        rejectUnauthorized: false
                    }
                });

                await testTransporter.sendMail({
                    from: customConfig.from,
                    to: customConfig.to,
                    subject: '🧪 Pulse Alert System - Test Email',
                    text: 'This is a test email from Pulse monitoring system.',
                    html: '<p>This is a test email from Pulse monitoring system.</p>'
                });

                testTransporter.close();
                return { success: true, provider: 'smtp' };
            }
        } else {
            // Test with current config
            await this.sendMail({
                to: this.config.to,
                from: this.config.from,
                subject: '🧪 Pulse Alert System - Test Email',
                text: 'This is a test email from Pulse monitoring system.',
                html: '<p>This is a test email from Pulse monitoring system.</p>'
            });
            return { success: true, provider: this.provider };
        }
    }

    /**
     * Close connections
     */
    close() {
        if (this.transporter) {
            this.transporter.close();
            this.transporter = null;
        }
    }

    /**
     * Get current provider
     */
    getProvider() {
        return this.provider;
    }

    /**
     * Get current configuration
     */
    getConfig() {
        return this.config;
    }
}

module.exports = EmailService;