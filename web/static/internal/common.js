const htmlTag = document.getElementsByTagName("html")[0];

const getCurrentTheme = () => {
  const storedTheme = getLocalStorageItem("theme");
  if (storedTheme) {
    return storedTheme;
  }
  
  const htmlTag = document.getElementsByTagName("html")[0];
  if (htmlTag.hasAttribute("data-theme")) {
    const themeAttr = htmlTag.getAttribute("data-theme");
    setLocalStorageItem("theme", themeAttr);
    return themeAttr;
  }
  
  // Return system theme preference
  const prefersDark = window.matchMedia && 
    window.matchMedia("(prefers-color-scheme: dark)").matches;
  const systemTheme = prefersDark ? "dark" : "light";
  setLocalStorageItem("theme", systemTheme);
  return systemTheme;
};

const toggleTheme = () => {
  const htmlTag = document.getElementsByTagName("html")[0];
  const newTheme = getCurrentTheme() === "dark" ? "light" : "dark";
  
  setLocalStorageItem("theme", newTheme);
  htmlTag.setAttribute("data-theme", newTheme);
};

const initializeTheme = () => {
  const elements = safeGetElementsById(["sunIcon", "moonIcon"]);
  const { sunIcon, moonIcon } = elements;

  if (getCurrentTheme() === "light") {
    const htmlTag = document.getElementsByTagName("html")[0];
    
    if (sunIcon && moonIcon) {
      sunIcon.classList.replace("swap-on", "swap-off");
      moonIcon.classList.replace("swap-off", "swap-on");
    }
    htmlTag.setAttribute("data-theme", "light");
  }
};

initializeTheme();
function isCatchupEnabled() {
  const v = getLocalStorageItem("catchupMode", false);
  return !!v;
}
function applyCatchupToCards() {
  const cards = document.querySelectorAll('a.card[data-channel-id]');
  const params = getCurrentUrlParams();
  const qs = params.toString();
  const suffix = qs ? '?' + qs : '';
  const base = isCatchupEnabled() ? '/catchup/' : '/play/';
  cards.forEach(card => {
    const id = card && card.getAttribute('data-channel-id');
    if (id) {
      card.setAttribute('href', base + id + suffix);
    }
  });
}
function styleCatchupCards() {
  const cards = document.querySelectorAll('a.card[data-channel-id]');
  const enabled = isCatchupEnabled();
  cards.forEach(card => {
    if (!card) return;
    if (enabled) {
      card.classList.remove('border-primary');
      card.classList.add('border-warning');
    } else {
      card.classList.remove('border-warning');
      card.classList.add('border-primary');
    }
  });
}
function updateCatchupUI() {
  const btn = document.getElementById("catchup-toggle");
  const label = document.getElementById("catchup-toggle-label");
  const enabled = isCatchupEnabled();
  if (btn) {
    if (typeof btn.checked !== "undefined") {
      btn.checked = enabled;
    } else {
      if (enabled) {
        btn.classList.add("btn-active");
      } else {
        btn.classList.remove("btn-active");
      }
    }
  }
  if (label) {
    label.textContent = enabled ? "Catchup: ON" : "Catchup: OFF";
  }
  applyCatchupToCards();
  styleCatchupCards();
  const path = window.location.pathname;
  const match = path.match(/^\/(play|catchup)\/([^/?#]+)/);
  if (match) {
    const id = match[2];
    const params = getCurrentUrlParams();
    const qs = params.toString();
    const base = enabled ? "/catchup/" : "/play/";
    const target = base + id + (qs ? "?" + qs : "");
    if ((enabled && match[1] !== "catchup") || (!enabled && match[1] !== "play")) {
      window.location.replace(target);
    }
  }
}
function toggleCatchupMode() {
  const btn = document.getElementById("catchup-toggle");
  let next = !isCatchupEnabled();
  if (btn && typeof btn.checked !== "undefined") {
    next = !!btn.checked;
  }
  setLocalStorageItem("catchupMode", next);
  updateCatchupUI();
  const path = window.location.pathname;
  const match = path.match(/^\/(play|catchup)\/([^/?#]+)/);
  if (match) {
    const id = match[2];
    const params = getCurrentUrlParams();
    const qs = params.toString();
    const base = next ? "/catchup/" : "/play/";
    const target = base + id + (qs ? "?" + qs : "");
    if ((next && match[1] !== "catchup") || (!next && match[1] !== "play")) {
      window.location.replace(target);
    }
  }
}
document.addEventListener('DOMContentLoaded', function() {
  updateCatchupUI();
});
