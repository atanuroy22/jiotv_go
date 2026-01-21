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
  const v = getLocalStorageItem("catchupMode", true);
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
  if (btn) {
    if (isCatchupEnabled()) {
      btn.classList.add("btn-active");
    } else {
      btn.classList.remove("btn-active");
    }
  }
  applyCatchupToCards();
  styleCatchupCards();
}
function toggleCatchupMode() {
  const next = !isCatchupEnabled();
  setLocalStorageItem("catchupMode", next);
  updateCatchupUI();
}
document.addEventListener('DOMContentLoaded', function() {
  updateCatchupUI();
});
