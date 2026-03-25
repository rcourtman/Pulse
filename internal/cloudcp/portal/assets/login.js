var emailEl = document.getElementById('email');
var sendBtn = document.getElementById('send-btn');
var successEl = document.getElementById('success');
var errEl = document.getElementById('err-msg');

async function sendMagicLink() {
  var email = emailEl.value.trim();
  if (!email) { emailEl.focus(); return; }
  sendBtn.disabled = true;
  sendBtn.textContent = 'Sending…';
  errEl.style.display = 'none';
  try {
    var r = await fetch('/api/public/magic-link/request', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: email })
    });
    if (r.ok || r.status === 404) {
      successEl.style.display = 'block';
      sendBtn.style.display = 'none';
      emailEl.style.display = 'none';
      document.querySelector('label').style.display = 'none';
    } else if (r.status === 429) {
      errEl.textContent = 'Too many requests. Please wait a moment and try again.';
      errEl.style.display = 'block';
      sendBtn.disabled = false;
      sendBtn.textContent = 'Send magic link';
    } else {
      errEl.textContent = 'Something went wrong. Please try again.';
      errEl.style.display = 'block';
      sendBtn.disabled = false;
      sendBtn.textContent = 'Send magic link';
    }
  } catch (_) {
    errEl.textContent = 'Network error. Please check your connection and try again.';
    errEl.style.display = 'block';
    sendBtn.disabled = false;
    sendBtn.textContent = 'Send magic link';
  }
}

sendBtn.onclick = sendMagicLink;
emailEl.addEventListener('keydown', function(e) { if (e.key === 'Enter') sendMagicLink(); });
document.getElementById('resend-link').onclick = function(e) {
  e.preventDefault();
  successEl.style.display = 'none';
  sendBtn.style.display = '';
  emailEl.style.display = '';
  document.querySelector('label').style.display = '';
  sendBtn.disabled = false;
  sendBtn.textContent = 'Send magic link';
};
