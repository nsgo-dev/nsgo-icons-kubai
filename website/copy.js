// Copies a code block's text when its button is pressed.
for (const button of document.querySelectorAll('.code button')) {
  const label = button.textContent;
  button.addEventListener('click', async () => {
    await navigator.clipboard.writeText(button.parentElement.querySelector('pre').textContent);
    button.textContent = button.dataset.copied;
    setTimeout(() => (button.textContent = label), 1500);
  });
}
