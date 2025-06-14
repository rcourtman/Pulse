(function setupHotReload() {
  const socket = io();

  socket.on('hotReload', function() {
    window.location.reload();
  });

  let wasConnected = false;
  socket.on('connect', function() {
    if (wasConnected) {
      setTimeout(() => window.location.reload(), 500);
    }
    wasConnected = true;
  });

  socket.on('disconnect', function(reason) {
    wasConnected = false;
  });

})(); 