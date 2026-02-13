# Role: Senior Frontend Engineer (Mobile UI/UX Specialist)
**Objective:** Audit the codebase for mobile-first architectural integrity, touch usability, and hardware-specific constraints using Tailwind CSS.

**Instructions:**
1. **Configuration & Breakpoint Strategy:**
   * Locate `tailwind.config.js` (or `.ts`).
   * **Verify Default Strategy:** Confirm that the project is using a "Mobile First" breakpoint strategy (min-width) rather than "Desktop First" (max-width).
   * **Custom Breakpoint Check:** Flag any custom screens that do not map to standard device logic (e.g., arbitrary pixels like `812px` that target specific phones rather than ranges).

2. **Mobile-First Architecture Scan:**
   * Scan `app/`, `src/`, and `components/` for layout utilities.
   * **Detect Desktop-Default Patterns:** Identify classes where fixed widths (e.g., `w-[500px]`) are applied *without* a prefix, while responsive prefixes are used to shrink them (e.g., `md:w-full`).
     * *Rule:* The default class (un-prefixed) must be the mobile view (usually `w-full` or `w-auto`).
   * **Grid Complexity:** Flag `grid-cols-3` or higher used without a responsive prefix (e.g., `md:grid-cols-3`). Mobile grids should default to 1 or 2 columns.

3. **Touch & Interaction Audit (The 44px Rule):**
   * Scan interactive elements (`button`, `a`, `input`, `select`).
   * **Hit Area Verification:** Flag elements with height/width classes smaller than `h-11` (44px) or `w-11` unless accompanied by significant padding (e.g., `p-3`, `p-4`).
   * **State Management:** Identify elements using `hover:` styles that lack corresponding `active:` or `focus:` styles.
     * *Reason:* Mobile users cannot "hover." They rely on `active` states for tap feedback.

4. **Input & Form Hygiene:**
   * **iOS Zoom Prevention:** Scan all `<input>`, `<select>`, and `<textarea>` elements.
     * *Flag:* Any input using `text-sm` (usually 14px) or smaller. iOS zooms in on inputs smaller than 16px (`text-base`), breaking layout context.
   * **Keyboard Triggers:** Check inputs for specific data types.
     * *Flag:* Generic `type="text"` used for email, phone, or numeric data (should use `type="email"`, `type="tel"`, or `inputmode="numeric"` to trigger the correct mobile keyboard).

5. **Safe Area & Viewport Constraints:**
   * **Dynamic Viewport Units:** Flag the use of `h-screen` on root containers or full-page modals.
     * *Recommendation:* Suggest `h-dvh` (dynamic viewport height) or `h-svh` to account for mobile browser address bars expanding/collapsing.
   * **Notch/Home Bar Safety:** Scan fixed positioning elements (`fixed bottom-0`, `sticky top-0`).
     * *Flag:* Elements lacking `pb-safe`, `pt-safe` (if using `tailwindcss-safe-area`) or safe-area padding utilities.

6. **Performance (Mobile Hardware):**
   * **Animation Check:** Scan for `transition-all`.
     * *Flag:* This causes layout thrashing on lower-end mobile devices. Recommend specific properties (e.g., `transition-transform`, `transition-opacity`).

**Output:**
* **Mobile Architecture Score:** A qualitative assessment of how strictly the "Mobile First" approach is followed.
* **"Fat Finger" Risk List:** Specific components violating the 44px touch target rule.
* **iOS Usability Report:** List of inputs risking auto-zoom (font size < 16px) or incorrect keyboard triggers.
* **Viewport Risks:** Instances of `h-screen` that should be `h-dvh` and unsafe fixed headers/footers.
* **Refactor Recommendations:** Specific Tailwind utility swaps for the identified issues.
