  expect(mockPveClientInstance.post).toHaveBeenCalledTimes(1);
  // Adjust expectation to match the actual log which includes the data object
  expect(consoleWarnSpy).toHaveBeenCalledWith(
    // expect.stringContaining("Guest agent memory command 'get-memory-block-info' response format not as expected"),
    // expect.stringContaining(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Guest agent mem format not as expected. Data:`),
    expect.stringContaining(`[Metrics Cycle - ${endpointName}] VM ${vmid} (${guestName}): Guest agent mem format not as expected (object structure). Data:`),
    expect.objectContaining({ result: { unexpected: "data" } }) // Check for the logged object too. This needs to match the second arg to console.warn now which is memInfo
  );
  consoleWarnSpy.mockRestore();
});

test('should use cached guest memory details if available and recent enough', async () => {
// ... existing code ...
} 