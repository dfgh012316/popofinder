// The site's only script (docs/open-decisions.md OD-17): 「複製 Email」 copies the
// contact address to the clipboard, as Slipkit does. It sends nothing anywhere.
// The buttons are hidden in the HTML and shown here, so without scripts only the
// mailto link and the address remain.
"use strict";
for (const b of document.querySelectorAll("button[data-copy]")) {
  b.hidden = false;
  b.addEventListener("click", async () => {
    const status = document.getElementById(b.getAttribute("aria-controls"));
    try {
      await navigator.clipboard.writeText(b.dataset.copy);
      status.textContent = "已複製 Email，可貼到你使用的信箱。";
    } catch {
      status.textContent = "無法自動複製，請長按或選取下方 Email 手動複製。";
    }
  });
}
