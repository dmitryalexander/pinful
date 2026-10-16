/* Hover actions share the existing click controls, including download cancellation and Lens routing. */
(() => {
  'use strict';
  let pointer = null;
  document.addEventListener('pointermove', event => {
    pointer = {x: event.clientX, y: event.clientY};
  }, {passive: true});
  document.addEventListener('pointerout', event => { if (!event.relatedTarget) pointer = null; }, {passive: true});
  window.addEventListener('blur', () => { pointer = null; });
  document.addEventListener('keydown', event => {
    if (event.defaultPrevented || event.repeat || event.ctrlKey || event.shiftKey || event.altKey || event.metaKey || event.isComposing) return;
    const typing = event.composedPath().some(element => element.isContentEditable || element.matches?.('input,textarea,select,[role="textbox"]'));
    if (typing || document.querySelector('dialog[open], [role="dialog"][aria-modal="true"], .pt-playlist-picker:not([hidden])')) return;
    const key = event.key.toLowerCase();
    if (!['g', 'o', 'd', ' '].includes(key)) return;
    // Hit-test now: scrolling, shuffling or removing a pin can move a new tile under the pointer.
    const target = pointer ? document.elementFromPoint(pointer.x, pointer.y) : document.activeElement;
    const tile = target?.closest('.pin-video-tile');
    if (!tile || tile.closest('[hidden]')) return;
    if (key === ' ' && event.target?.closest('button,a,video') && !tile.contains(event.target)) return;
    let action;
    if (key === 'g') action = tile.querySelector('.pin-lens-open');
    if (key === 'o') action = tile.querySelector('a.pin-image-link[href^="/pin/"], a.pin-media-open[href^="/pin/"]');
    if (key === 'd') action = tile.querySelector('.pin-video-download');
    if (key !== ' ') {
      if (!action || action.disabled) return;
      event.preventDefault();
      action.click();
      return;
    }
    const video = tile.querySelector('video');
    const play = tile.querySelector('.pin-video-play');
    if (!video || !play || play.disabled) return;
    event.preventDefault();
    if (!tile.classList.contains('is-playing')) play.click();
    else if (!video.paused) video.pause();
    else video.play().catch(() => {
      const status = tile.querySelector('.pin-video-status');
      if (status) { status.textContent = 'Click the video to resume playback.'; status.hidden = false; }
    });
  });
})();
