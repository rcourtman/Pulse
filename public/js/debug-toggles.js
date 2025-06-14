// Debug function for notification toggles
// Copy and paste this into the browser console when the alert modal is open

function debugToggles() {
    console.log('=== Toggle Debug Info ===');
    
    // Check if modal is open
    const modal = document.getElementById('alert-management-modal');
    console.log('Modal found:', !!modal);
    console.log('Modal visible:', modal && !modal.classList.contains('hidden'));
    
    // Check active tab
    const activeTab = document.querySelector('.alert-tab.active');
    console.log('Active tab:', activeTab ? activeTab.getAttribute('data-tab') : 'none');
    
    // Check toggle elements
    const emailToggle = document.getElementById('global-email-toggle');
    const webhookToggle = document.getElementById('global-webhook-toggle');
    
    console.log('\n--- Toggle Elements ---');
    console.log('Email toggle found:', !!emailToggle);
    if (emailToggle) {
        console.log('Email toggle checked:', emailToggle.checked);
        console.log('Email toggle disabled:', emailToggle.disabled);
        console.log('Email toggle parent:', emailToggle.parentElement);
    }
    
    console.log('\nWebhook toggle found:', !!webhookToggle);
    if (webhookToggle) {
        console.log('Webhook toggle checked:', webhookToggle.checked);
        console.log('Webhook toggle disabled:', webhookToggle.disabled);
        console.log('Webhook toggle parent:', webhookToggle.parentElement);
    }
    
    // Check config sections
    const emailSection = document.getElementById('email-config-section');
    const webhookSection = document.getElementById('webhook-config-section');
    
    console.log('\n--- Config Sections ---');
    console.log('Email section found:', !!emailSection);
    if (emailSection) {
        console.log('Email section opacity:', emailSection.style.opacity);
        console.log('Email section inputs:', emailSection.querySelectorAll('input').length);
    }
    
    console.log('\nWebhook section found:', !!webhookSection);
    if (webhookSection) {
        console.log('Webhook section opacity:', webhookSection.style.opacity);
        console.log('Webhook section inputs:', webhookSection.querySelectorAll('input').length);
    }
    
    // Check current config
    console.log('\n--- Current Config ---');
    if (window.PulseApp && window.PulseApp.ui && window.PulseApp.ui.alertManagementModal) {
        console.log('PulseApp.ui.alertManagementModal available');
    }
    
    // Test toggle click
    console.log('\n--- Testing Toggle Click ---');
    if (emailToggle) {
        console.log('Simulating email toggle click...');
        const event = new Event('change', { bubbles: true });
        emailToggle.checked = !emailToggle.checked;
        emailToggle.dispatchEvent(event);
    }
}

// Run the debug function
debugToggles();

// Additional helper to manually trigger toggle setup
function manualSetupToggles() {
    if (window.PulseApp && window.PulseApp.ui && window.PulseApp.ui.alertManagementModal) {
        // Try to access the setupNotificationToggles function if it's exposed
        console.log('Attempting manual toggle setup...');
        
        // Manually recreate the setup logic
        const emailToggle = document.getElementById('global-email-toggle');
        const webhookToggle = document.getElementById('global-webhook-toggle');
        
        if (emailToggle) {
            emailToggle.addEventListener('change', (e) => {
                console.log('[Manual] Email toggle changed to:', e.target.checked);
            });
        }
        
        if (webhookToggle) {
            webhookToggle.addEventListener('change', (e) => {
                console.log('[Manual] Webhook toggle changed to:', e.target.checked);
            });
        }
    }
}