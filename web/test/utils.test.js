// Set up TextEncoder/TextDecoder for JSDOM compatibility
const { TextEncoder, TextDecoder } = require('util');
global.TextEncoder = TextEncoder;
global.TextDecoder = TextDecoder;

// We'll use the real JSDOM localStorage and attach spies when needed

// Mock fetch for testing
global.fetch = jest.fn();

// Use real history; we'll spy on replaceState (so underlying implementation still runs)
// We'll attach the spy after utils are loaded.

// Ensure a clean localStorage state at file start
localStorage.clear();

// Load the utility functions by evaluating the file content
const fs = require('fs');
const path = require('path');
const utilsScript = fs.readFileSync(path.join(__dirname, '..', 'static', 'internal', 'utils.js'), 'utf8');

// Remove the module.exports part and evaluate the functions
const scriptWithoutExports = utilsScript.replace(/if \(typeof module.*\{[\s\S]*\}/, '');
eval(scriptWithoutExports);

describe('Utility Functions', () => {
  // Attach spy once so we preserve original behavior
  if (!jest.isMockFunction(window.history.replaceState)) {
    jest.spyOn(window.history, 'replaceState');
  }

  beforeEach(() => {
    // Clear mocks usage data
    jest.clearAllMocks();
    localStorage.clear();
    // Rebuild DOM elements required for tests
    document.body.innerHTML = `
      <div id="test-element">Test Content</div>
      <div id="element-1"></div>
      <div id="element-2"></div>
      <button id="favorite-btn-123" class="btn"></button>
      <span id="star-icon-123"></span>
      <span id="x-icon-123" class="hidden"></span>
    `;
    // Set initial URL state using pushState so window.location is actually updated
    window.history.pushState({}, '', '/channels?search=test&category=sports');
  });
  // NOTE: We intentionally do NOT call jest.resetAllMocks() because it removes
  // mock implementations (like document.getElementById, history.replaceState,
  // and localStorage methods) that the tests depend on across cases. We only
  // clear call history so each test starts fresh while retaining behavior.

  describe('DOM Utilities', () => {
    describe('safeGetElementById', () => {
      it('should return element when it exists', () => {
        const element = safeGetElementById('test-element');
        expect(element).toBeTruthy();
        expect(element.textContent).toBe('Test Content');
      });

      it('should return null when element does not exist', () => {
        const element = safeGetElementById('non-existent');
        expect(element).toBeNull();
      });

      it('should log warning when element not found and suppressError is false', () => {
        const consoleSpy = jest.spyOn(console, 'warn').mockImplementation();
        safeGetElementById('non-existent');
        expect(consoleSpy).toHaveBeenCalledWith("Element with ID 'non-existent' not found");
        consoleSpy.mockRestore();
      });

      it('should not log warning when suppressError is true', () => {
        const consoleSpy = jest.spyOn(console, 'warn').mockImplementation();
        safeGetElementById('non-existent', true);
        expect(consoleSpy).not.toHaveBeenCalled();
        consoleSpy.mockRestore();
      });
    });

    describe('safeGetElementsById', () => {
      it('should return object with all requested elements', () => {
        const elements = safeGetElementsById(['test-element', 'element-1', 'element-2']);
        expect(elements['test-element']).toBeTruthy();
        expect(elements['element-1']).toBeTruthy();
        expect(elements['element-2']).toBeTruthy();
        expect(elements['test-element'].textContent).toBe('Test Content');
      });

      it('should include null values for non-existent elements', () => {
        const elements = safeGetElementsById(['test-element', 'non-existent']);
        expect(elements['test-element']).toBeTruthy();
        expect(elements['non-existent']).toBeNull();
      });
    });

    describe('createElement', () => {
      it('should create element with basic attributes', () => {
        const element = createElement('div', { id: 'new-div', className: 'test-class' }, 'Test content');
        expect(element.tagName).toBe('DIV');
        expect(element.id).toBe('new-div');
        expect(element.className).toBe('test-class');
        expect(element.textContent).toBe('Test content');
      });

      it('should create element with innerHTML when provided', () => {
        const element = createElement('div', {}, 'Text content', '<span>HTML content</span>');
        expect(element.innerHTML).toBe('<span>HTML content</span>');
        expect(element.textContent).toBe('HTML content');
      });

      it('should create element with custom attributes', () => {
        const element = createElement('a', {
          href: '/test',
          'data-channel-id': '123',
          tabindex: '0'
        });
        expect(element.getAttribute('href')).toBe('/test');
        expect(element.getAttribute('data-channel-id')).toBe('123');
        expect(element.getAttribute('tabindex')).toBe('0');
      });
    });
  });

  describe('CSS Class Utilities', () => {
    describe('toggleClasses', () => {
      it('should add and remove classes based on condition (true)', () => {
        const element = document.getElementById('test-element');
        toggleClasses(element, 'active', 'inactive', true);
        expect(element.classList.contains('active')).toBe(true);
        expect(element.classList.contains('inactive')).toBe(false);
      });

      it('should add and remove classes based on condition (false)', () => {
        const element = document.getElementById('test-element');
        element.classList.add('active');
        toggleClasses(element, 'active', 'inactive', false);
        expect(element.classList.contains('active')).toBe(false);
        expect(element.classList.contains('inactive')).toBe(true);
      });

      it('should handle null element gracefully', () => {
        expect(() => toggleClasses(null, 'class1', 'class2', true)).not.toThrow();
      });
    });

    describe('setElementVisibility', () => {
      it('should remove hidden class when visible is true', () => {
        const element = document.getElementById('test-element');
        element.classList.add('hidden');
        setElementVisibility(element, true);
        expect(element.classList.contains('hidden')).toBe(false);
      });

      it('should add hidden class when visible is false', () => {
        const element = document.getElementById('test-element');
        setElementVisibility(element, false);
        expect(element.classList.contains('hidden')).toBe(true);
      });
    });
  });

  describe('LocalStorage Utilities', () => {
    describe('getLocalStorageItem', () => {
      it('should return parsed JSON value when item exists', () => {
        localStorage.setItem('test-key', JSON.stringify({ name: 'test' }));
        const result = getLocalStorageItem('test-key');
        expect(result).toEqual({ name: 'test' });
      });

      it('should return default value when item does not exist', () => {
        const result = getLocalStorageItem('non-existent', 'default');
        expect(result).toBe('default');
      });

      it('should return default value when JSON parsing fails', () => {
        localStorage.setItem('invalid-json', 'invalid json');
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation();
        const result = getLocalStorageItem('invalid-json', 'default');
        expect(result).toBe('default');
        expect(consoleSpy).toHaveBeenCalled();
        consoleSpy.mockRestore();
      });
    });

    describe('setLocalStorageItem', () => {
      it('should store value as JSON string', () => {
        const result = setLocalStorageItem('test-key', { name: 'test' });
        expect(result).toBe(true);
        expect(localStorage.getItem('test-key')).toBe('{"name":"test"}');
      });

      it('should handle storage errors gracefully', () => {
        const setSpy = jest.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
          throw new Error('Storage quota exceeded');
        });
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation();
        const result = setLocalStorageItem('test-key', 'value');
        expect(result).toBe(false);
        expect(consoleSpy).toHaveBeenCalled();
        consoleSpy.mockRestore();
        setSpy.mockRestore();
      });
    });

    describe('removeLocalStorageItem', () => {
      it('should remove item from localStorage', () => {
        localStorage.setItem('test-key', '123');
        const result = removeLocalStorageItem('test-key');
        expect(result).toBe(true);
        expect(localStorage.getItem('test-key')).toBeNull();
      });
    });
  });

  describe('URL Utilities', () => {
    describe('getCurrentUrlParams', () => {
      it('should return URLSearchParams object', () => {
        const params = getCurrentUrlParams();
        expect(params).toBeInstanceOf(window.URLSearchParams);
        expect(params.get('search')).toBe('test');
        expect(params.get('category')).toBe('sports');
      });
    });

    describe('updateUrlParameter', () => {
      it('should update existing parameter', () => {
        updateUrlParameter('search', 'new-search');
        const calls = window.history.replaceState.mock.calls;
        const last = calls[calls.length - 1];
        const base = window.location.pathname === '/channels' ? '/channels' : '/';
        expect(last).toEqual([{}, '', `${base}?search=new-search&category=sports`]);
      });

      it('should add new parameter', () => {
        updateUrlParameter('language', 'english');
        const calls = window.history.replaceState.mock.calls;
        const last = calls[calls.length - 1];
        const base = window.location.pathname === '/channels' ? '/channels' : '/';
        expect(last).toEqual([{}, '', `${base}?search=test&category=sports&language=english`]);
      });

      it('should remove parameter when value is empty', () => {
        updateUrlParameter('search', '');
        const calls = window.history.replaceState.mock.calls;
        const last = calls[calls.length - 1];
        const base = window.location.pathname === '/channels' ? '/channels' : '/';
        expect(last).toEqual([{}, '', `${base}?category=sports`]);
      });

      it('should use custom replaceState function when provided', () => {
        const customReplace = jest.fn();
        updateUrlParameter('search', 'custom', customReplace);
        expect(customReplace).toHaveBeenCalledWith(
          {},
          '',
          '/channels?search=custom&category=sports'
        );
      });
    });

    describe('updateUrlParameters', () => {
      it('should update multiple parameters', () => {
        updateUrlParameters({
          search: 'new-search',
          category: 'news',
          language: 'english'
        });
        const calls = window.history.replaceState.mock.calls;
        const last = calls[calls.length - 1];
        const base = window.location.pathname === '/channels' ? '/channels' : '/';
        expect(last).toEqual([{}, '', `${base}?search=new-search&category=news&language=english`]);
      });
    });
  });

  describe('Fetch Utilities', () => {
    describe('postJSON', () => {
      it('should make POST request with JSON body', async () => {
        const mockResponse = { status: 'success' };
        fetch.mockResolvedValueOnce({
          json: () => Promise.resolve(mockResponse)
        });

        const result = await postJSON('/api/test', { name: 'test' });
        
        expect(fetch).toHaveBeenCalledWith('/api/test', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: '{"name":"test"}'
        });
        expect(result).toEqual(mockResponse);
      });

      it('should handle fetch errors', async () => {
        fetch.mockRejectedValueOnce(new Error('Network error'));
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation();

        await expect(postJSON('/api/test', {})).rejects.toThrow('Network error');
        expect(consoleSpy).toHaveBeenCalled();
        consoleSpy.mockRestore();
      });
    });

    describe('getJSON', () => {
      it('should make GET request and return JSON', async () => {
        const mockResponse = { data: 'test' };
        fetch.mockResolvedValueOnce({
          ok: true,
          json: () => Promise.resolve(mockResponse)
        });

        const result = await getJSON('/api/test');
        
        expect(fetch).toHaveBeenCalledWith('/api/test', {});
        expect(result).toEqual(mockResponse);
      });

      it('should handle HTTP errors', async () => {
        fetch.mockResolvedValueOnce({
          ok: false,
          status: 404
        });
        const consoleSpy = jest.spyOn(console, 'error').mockImplementation();

        await expect(getJSON('/api/test')).rejects.toThrow('HTTP error! status: 404');
        expect(consoleSpy).toHaveBeenCalled();
        consoleSpy.mockRestore();
      });
    });
  });

  describe('Icon and Button Utilities', () => {
    describe('updateFavoriteButtonState', () => {
      it('should update button state when favorited', () => {
        updateFavoriteButtonState('123', true);
        
        const button = document.getElementById('favorite-btn-123');
        const starIcon = document.getElementById('star-icon-123');
        const xIcon = document.getElementById('x-icon-123');
        
        expect(button.classList.contains('favorited')).toBe(true);
        expect(starIcon.classList.contains('hidden')).toBe(true);
        expect(xIcon.classList.contains('hidden')).toBe(false);
      });

      it('should update button state when not favorited', () => {
        const button = document.getElementById('favorite-btn-123');
        const starIcon = document.getElementById('star-icon-123');
        const xIcon = document.getElementById('x-icon-123');
        
        // Set initial favorited state
        button.classList.add('favorited');
        starIcon.classList.add('hidden');
        xIcon.classList.remove('hidden');
        
        updateFavoriteButtonState('123', false);
        
        expect(button.classList.contains('favorited')).toBe(false);
        expect(starIcon.classList.contains('hidden')).toBe(false);
        expect(xIcon.classList.contains('hidden')).toBe(true);
      });

      it('should handle missing elements gracefully', () => {
        expect(() => updateFavoriteButtonState('non-existent', true)).not.toThrow();
      });
    });
  });
});