// Delete mapping confirmation
document.addEventListener('DOMContentLoaded', function() {
  const deleteForms = document.querySelectorAll('.delete-form');

  deleteForms.forEach(form => {
    form.addEventListener('submit', function(e) {
      e.preventDefault();

      // Get the media title from the aria-label of the button
      const button = form.querySelector('.delete-btn');
      const ariaLabel = button.getAttribute('aria-label');
      const mediaTitle = ariaLabel.replace('Delete ', '');

      // Show confirmation dialog
      const confirmed = confirm(`Are you sure you want to delete the mapping for "${mediaTitle}"?\n\nThis cannot be undone.`);

      if (confirmed) {
        // Submit the form
        form.submit();
      }
    });
  });
});
