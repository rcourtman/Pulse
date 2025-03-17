import puppeteer, { Browser, Page } from 'puppeteer';
import fs from 'fs';
import path from 'path';
import { ScreenshotConfig, ViewportSize, CropRegion, SplitViewConfig } from './types';
import { logger } from './logger';
import sharp from 'sharp';
import os from 'os';

// Define window interface to include applyTheme
declare global {
  interface Window {
    applyTheme?: () => void;
  }
}

class ScreenshotTool {
  private browser: Browser | null = null;
  private config: ScreenshotConfig;
  private baseUrl: string;
  private outputDir: string;
  
  constructor(configPath: string) {
    // Load configuration
    this.config = this.loadConfig(configPath);
    this.baseUrl = this.config.baseUrl || 'http://localhost:7656';
    this.outputDir = this.config.outputDir || path.join(process.cwd(), 'docs', 'images');
    
    // Ensure output directory exists
    if (!fs.existsSync(this.outputDir)) {
      fs.mkdirSync(this.outputDir, { recursive: true });
      logger.info(`Created output directory: ${this.outputDir}`);
    }
  }
  
  private loadConfig(configPath: string): ScreenshotConfig {
    try {
      const configFile = fs.readFileSync(configPath, 'utf8');
      return JSON.parse(configFile);
    } catch (error) {
      logger.error(`Failed to load config file: ${error}`);
      throw new Error(`Failed to load config file: ${error}`);
    }
  }
  
  setBaseUrl(url: string): void {
    this.baseUrl = url;
  }
  
  setOutputDir(dir: string): void {
    this.outputDir = dir;
    // Ensure output directory exists
    if (!fs.existsSync(this.outputDir)) {
      fs.mkdirSync(this.outputDir, { recursive: true });
      logger.info(`Created output directory: ${this.outputDir}`);
    }
  }
  
  async initialize(): Promise<void> {
    this.browser = await puppeteer.launch({
      headless: true,
      defaultViewport: null,
      args: ['--no-sandbox', '--disable-setuid-sandbox']
    });
    logger.info('Browser initialized');
    
    // We'll set up mock data for each page individually instead of globally
  }
  
  async close(): Promise<void> {
    if (this.browser) {
      await this.browser.close();
      this.browser = null;
      logger.info('Browser closed');
    }
  }
  
  async takeScreenshot(
    pagePath: string, 
    outputName: string, 
    viewportSize: ViewportSize = { width: 1440, height: 900 },
    theme: 'light' | 'dark' = 'light',
    waitForSelector?: string,
    cropRegion?: CropRegion,
    enableFilters: boolean = true,
    beforeScreenshot?: string
  ): Promise<string> {
    if (!this.browser) {
      throw new Error('Browser not initialized. Call initialize() first.');
    }
    
    const page = await this.browser.newPage();
    
    // Set viewport size with deviceScaleFactor of 2 for Retina-quality screenshots
    await page.setViewport({
      width: viewportSize.width,
      height: viewportSize.height,
      deviceScaleFactor: 2  // This is key for high-quality screenshots on Retina displays
    });
    
    // Setup mock data if configured
    if (this.config.mockData?.enabled) {
      await this.setupMockData(page);
    }
    
    // Set theme first before navigating to ensure it's applied on initial load
    if (theme === 'dark') {
      // Set dark mode in localStorage before navigation
      await page.evaluateOnNewDocument(() => {
        localStorage.setItem('app_dark_mode', JSON.stringify(true));
        
        // Check if we have a saved filter state and restore it
        try {
          const savedFilterState = localStorage.getItem('app_filter_state');
          if (savedFilterState) {
            // Keep the saved filter state
            console.log('Restoring saved filter state:', savedFilterState);
          }
        } catch (e) {
          console.error('Error restoring filter state:', e);
        }
      });
    } else {
      // Set light mode in localStorage before navigation
      await page.evaluateOnNewDocument(() => {
        localStorage.setItem('app_dark_mode', JSON.stringify(false));
        
        // Check if we have a saved filter state and restore it
        try {
          const savedFilterState = localStorage.getItem('app_filter_state');
          if (savedFilterState) {
            // Keep the saved filter state
            console.log('Restoring saved filter state:', savedFilterState);
          }
        } catch (e) {
          console.error('Error restoring filter state:', e);
        }
      });
    }
    
    // Navigate to the page
    const url = `${this.baseUrl}${pagePath}`;
    logger.info(`Navigating to ${url} in ${theme} mode`);
    await page.goto(url, { waitUntil: 'networkidle2' });
    
    // Wait for specific element if needed
    if (waitForSelector) {
      await page.waitForSelector(waitForSelector, { visible: true });
    }
    
    // Add a small delay to ensure everything is loaded
    await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 2000)));
    
    // Verify and enforce theme if needed
    if (theme === 'dark') {
      await this.setDarkMode(page);
    } else {
      await this.setLightMode(page);
    }
    
    // Wait for data to load - look for elements that indicate data is loaded
    try {
      // Wait for any loading indicators to disappear
      await page.evaluate(() => {
        return new Promise((resolve) => {
          // Check if there are any loading indicators
          const checkForLoadingIndicators = () => {
            const loadingElements = document.querySelectorAll('.loading-indicator, [data-loading="true"], .MuiCircularProgress-root');
            if (loadingElements.length === 0) {
              resolve(true);
            } else {
              setTimeout(checkForLoadingIndicators, 500);
            }
          };
          
          // Start checking
          checkForLoadingIndicators();
          
          // Resolve anyway after a timeout to prevent hanging
          setTimeout(() => resolve(true), 5000);
        });
      });
      
      // Wait for data elements to appear
      await page.evaluate(() => {
        return new Promise((resolve) => {
          // Check if there are data elements
          const checkForDataElements = () => {
            // Look for elements that would indicate data is loaded
            const dataElements = document.querySelectorAll('.resource-card, .vm-card, .container-card, .guest-row, [data-testid="resource-item"]');
            if (dataElements.length > 0) {
              console.log(`Found ${dataElements.length} data elements`);
              resolve(true);
            } else {
              setTimeout(checkForDataElements, 500);
            }
          };
          
          // Start checking
          checkForDataElements();
          
          // Resolve anyway after a timeout to prevent hanging
          setTimeout(() => {
            console.log('Timed out waiting for data elements');
            resolve(true);
          }, 5000);
        });
      });
    } catch (error) {
      logger.warn(`Error waiting for data to load: ${error}`);
    }
    
    // Execute beforeScreenshot script if provided
    if (beforeScreenshot) {
      try {
        logger.info(`Executing beforeScreenshot script`);
        await page.evaluate((script) => {
          // eslint-disable-next-line no-eval
          return eval(script);
        }, beforeScreenshot);
        
        // Wait a moment for any UI changes to take effect
        await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 1000)));
      } catch (error) {
        logger.warn(`Error executing beforeScreenshot script: ${error}`);
      }
    }
    
    // Wait for data to load - look for elements that indicate data is loaded
    try {
      // Wait for any loading indicators to disappear
      await page.evaluate(() => {
        return new Promise((resolve) => {
          // Check if there are any loading indicators
          const checkForLoadingIndicators = () => {
            const loadingElements = document.querySelectorAll('.loading-indicator, [data-loading="true"], .MuiCircularProgress-root');
            if (loadingElements.length === 0) {
              resolve(true);
            } else {
              setTimeout(checkForLoadingIndicators, 500);
            }
          };
          
          // Start checking
          checkForLoadingIndicators();
          
          // Resolve anyway after a timeout to prevent hanging
          setTimeout(() => resolve(true), 5000);
        });
      });
      
      // Wait for data elements to appear
      await page.evaluate(() => {
        return new Promise((resolve) => {
          // Check if there are data elements
          const checkForDataElements = () => {
            // Look for elements that would indicate data is loaded
            const dataElements = document.querySelectorAll('.resource-card, .vm-card, .container-card, .guest-row, [data-testid="resource-item"]');
            if (dataElements.length > 0) {
              console.log(`Found ${dataElements.length} data elements`);
              resolve(true);
            } else {
              setTimeout(checkForDataElements, 500);
            }
          };
          
          // Start checking
          checkForDataElements();
          
          // Resolve anyway after a timeout to prevent hanging
          setTimeout(() => {
            console.log('Timed out waiting for data elements');
            resolve(true);
          }, 5000);
        });
      });
    } catch (error) {
      logger.warn(`Error waiting for data to load: ${error}`);
    }
    
    // Check if we should toggle filters or use the saved state
    if (enableFilters) {
      // Check if we have a saved filter state
      const hasSavedFilterState = await page.evaluate(() => {
        return localStorage.getItem('app_filter_state') !== null;
      });
      
      if (!hasSavedFilterState) {
        // No saved state, toggle filters as requested
        await this.toggleFilters(page, true);
      } else {
        // We have a saved state, check if it matches what we want
        const filterStateMatches = await page.evaluate(() => {
          try {
            const savedState = JSON.parse(localStorage.getItem('app_filter_state') || '{}');
            return savedState.filtersEnabled === true;
          } catch (e) {
            console.error('Error parsing filter state:', e);
            return false;
          }
        });
        
        if (!filterStateMatches) {
          // Saved state doesn't match what we want, toggle filters
          await this.toggleFilters(page, true);
        } else {
          logger.info('Using saved filter state (enabled)');
        }
      }
    } else {
      // We want filters disabled
      const hasSavedFilterState = await page.evaluate(() => {
        return localStorage.getItem('app_filter_state') !== null;
      });
      
      if (hasSavedFilterState) {
        // Check if saved state is already disabled
        const filterStateMatches = await page.evaluate(() => {
          try {
            const savedState = JSON.parse(localStorage.getItem('app_filter_state') || '{}');
            return savedState.filtersEnabled === false;
          } catch (e) {
            console.error('Error parsing filter state:', e);
            return false;
          }
        });
        
        if (!filterStateMatches) {
          // Saved state doesn't match what we want (disabled), toggle filters
          await this.toggleFilters(page, false);
        } else {
          logger.info('Using saved filter state (disabled)');
        }
      }
    }
    
    // Add a small delay to ensure everything is rendered after theme and filter changes
    await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 2000)));
    
    // Take screenshot
    const outputPath = path.join(this.outputDir, `${outputName}.png`);
    
    if (cropRegion) {
      // Take a screenshot of a specific region
      await page.screenshot({
        path: outputPath,
        clip: {
          x: cropRegion.x,
          y: cropRegion.y,
          width: cropRegion.width,
          height: cropRegion.height
        },
        omitBackground: false
      });
    } else {
      // Take a full page screenshot
      await page.screenshot({
        path: outputPath,
        fullPage: false,
        omitBackground: false
      });
    }
    
    logger.info(`Screenshot saved to ${outputPath}`);
    
    await page.close();
    return outputPath;
  }
  
  private async setDarkMode(page: Page): Promise<void> {
    logger.info('Setting dark mode');
    
    await page.evaluate(() => {
      localStorage.setItem('app_dark_mode', JSON.stringify(true));
      localStorage.setItem('use_mock_data', 'true');
      localStorage.setItem('MOCK_DATA_ENABLED', 'true');
      
      // Remove client-side mock data settings
      localStorage.removeItem('mock_enabled');
      localStorage.removeItem('MOCK_SERVER_URL');
      localStorage.removeItem('MOCK_DATA');
      
      // Apply theme if the function exists
      if (window.applyTheme) {
        window.applyTheme();
      }
    });
  }
  
  private async setLightMode(page: Page): Promise<void> {
    try {
      // Based on the application's ThemeContext implementation
      await page.evaluate(() => {
        // Set light mode in localStorage
        localStorage.setItem('app_dark_mode', JSON.stringify(false));
        
        // Force reload the page to ensure theme is applied
        window.location.reload();
      });
      
      // Wait for page to reload and stabilize
      await page.waitForNavigation({ waitUntil: 'networkidle2' });
      await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 2000)));
      
      // Verify light mode is applied
      const isLightMode = await page.evaluate(() => {
        // Check if body has light mode classes or computed styles
        const bodyStyles = window.getComputedStyle(document.body);
        const backgroundColor = bodyStyles.backgroundColor;
        // Light backgrounds typically have high RGB values
        const isLight = backgroundColor.includes('rgb(248, 249, 250)') || 
                       backgroundColor.includes('rgb(255, 255, 255)') ||
                       backgroundColor.includes('rgba(248, 249, 250)') ||
                       document.documentElement.classList.contains('light-mode');
        
        console.log('Current background color:', backgroundColor);
        return isLight;
      });
      
      if (isLightMode) {
        logger.info('Light mode successfully applied');
      } else {
        logger.warn('Light mode may not have been applied correctly');
        
        // Try an alternative approach - click the theme toggle button if available
        try {
          // Look for theme toggle button
          const themeToggleButton = await page.$('button[aria-label*="dark mode" i], button[aria-label*="light mode" i]');
          if (themeToggleButton) {
            logger.info('Found theme toggle button, clicking it');
            await themeToggleButton.click();
            await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 1000)));
          }
        } catch (error) {
          logger.warn(`Error trying to click theme toggle: ${error}`);
        }
      }
    } catch (error) {
      logger.error(`Error setting light mode: ${error}`);
    }
  }
  
  // Add a new method to toggle filters
  private async toggleFilters(page: Page, enable: boolean = true): Promise<void> {
    try {
      // First, check the current filter state
      const currentFilterState = await page.evaluate(() => {
        try {
          const savedState = localStorage.getItem('app_filter_state');
          if (savedState) {
            const parsed = JSON.parse(savedState);
            return parsed.filtersEnabled;
          }
        } catch (e) {
          console.error('Error checking filter state:', e);
        }
        return null; // Unknown state
      });
      
      // If the current state matches what we want, do nothing
      if (currentFilterState === enable) {
        logger.info(`Filters already ${enable ? 'enabled' : 'disabled'}, no action needed`);
        return;
      }
      
      // Look for filter buttons or toggles and click them
      if (enable) {
        // Try a more specific approach for this application
        await page.evaluate(() => {
          // Save the desired filter state to localStorage
          localStorage.setItem('app_filter_state', JSON.stringify({
            filtersEnabled: true,
            timestamp: Date.now()
          }));
          
          // Try to find and click filter elements using more specific selectors
          // Look for common filter UI elements
          const filterElements = [
            // Common filter button selectors
            ...document.querySelectorAll('.filter, .filters, [data-filter], [aria-label*="filter"]'),
            // Buttons with filter text
            ...Array.from(document.querySelectorAll('button')).filter(el => 
              el.textContent?.toLowerCase().includes('filter')
            ),
            // Filter dropdowns
            ...document.querySelectorAll('select[name*="filter"], .dropdown-filter'),
            // Filter checkboxes
            ...document.querySelectorAll('input[type="checkbox"][name*="filter"]'),
            // Filter toggles
            ...document.querySelectorAll('.toggle, .switch')
          ];
          
          // Click each filter element
          filterElements.forEach(el => {
            if (el instanceof HTMLElement) {
              console.log('Clicking filter element to enable:', el.outerHTML);
              el.click();
            }
          });
          
          // If no specific filters found, try to find elements by common class names
          if (filterElements.length === 0) {
            // Try to find and toggle any status filters
            const statusFilters = [
              ...document.querySelectorAll('[data-status], .status-filter, .status-toggle'),
              ...Array.from(document.querySelectorAll('button, .chip, .tag')).filter(el => 
                el.textContent?.toLowerCase().match(/status|running|stopped|online|offline/)
              )
            ];
            
            statusFilters.forEach(el => {
              if (el instanceof HTMLElement) {
                console.log('Clicking status filter to enable:', el.outerHTML);
                el.click();
              }
            });
          }
        });
        
        // Wait for any filter changes to apply
        await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 1000)));
        
        logger.info('Attempted to enable filters');
      } else {
        // Disable filters
        await page.evaluate(() => {
          // Save the desired filter state to localStorage
          localStorage.setItem('app_filter_state', JSON.stringify({
            filtersEnabled: false,
            timestamp: Date.now()
          }));
          
          // Try to find and click filter elements that are currently active
          const activeFilterElements = [
            // Active filter buttons
            ...document.querySelectorAll('.filter.active, .filters.active, [data-filter].active, [aria-label*="filter"].active'),
            // Active checkboxes
            ...Array.from(document.querySelectorAll('input[type="checkbox"][name*="filter"]:checked')),
            // Active toggles
            ...document.querySelectorAll('.toggle.active, .switch.active')
          ];
          
          // Click each active filter element to disable it
          activeFilterElements.forEach(el => {
            if (el instanceof HTMLElement) {
              console.log('Clicking filter element to disable:', el.outerHTML);
              el.click();
            }
          });
          
          // If no specific active filters found, try to find elements by common class names
          if (activeFilterElements.length === 0) {
            // Try to find and toggle any active status filters
            const activeStatusFilters = [
              ...document.querySelectorAll('[data-status].active, .status-filter.active, .status-toggle.active'),
              ...Array.from(document.querySelectorAll('button.active, .chip.active, .tag.active')).filter(el => 
                el.textContent?.toLowerCase().match(/status|running|stopped|online|offline/)
              )
            ];
            
            activeStatusFilters.forEach(el => {
              if (el instanceof HTMLElement) {
                console.log('Clicking active status filter to disable:', el.outerHTML);
                el.click();
              }
            });
          }
        });
        
        // Wait for any filter changes to apply
        await page.evaluate(() => new Promise(resolve => setTimeout(resolve, 1000)));
        
        logger.info('Attempted to disable filters');
      }
    } catch (error) {
      logger.warn(`Error toggling filters: ${error}`);
    }
  }
  
  async createSplitView(
    lightImagePath: string,
    darkImagePath: string,
    outputName: string,
    splitConfig: SplitViewConfig = { type: 'diagonal' }
  ): Promise<string> {
    const outputPath = path.join(this.outputDir, `${outputName}.png`);
    
    try {
      // We'll implement this using sharp
      const sharp = require('sharp');
      const lightBuffer = await fs.promises.readFile(lightImagePath);
      const darkBuffer = await fs.promises.readFile(darkImagePath);
      
      // Get image dimensions
      const lightImage = sharp(lightBuffer);
      const metadata = await lightImage.metadata();
      const { width = 1920, height = 1080 } = metadata;
      
      if (splitConfig.type === 'vertical') {
        // Create a vertical split (left: light, right: dark)
        const halfWidth = Math.floor(width / 2);
        
        // Extract left half from light image
        const leftHalf = await sharp(lightBuffer)
          .extract({ left: 0, top: 0, width: halfWidth, height })
          .toBuffer();
        
        // Extract right half from dark image
        const rightHalf = await sharp(darkBuffer)
          .extract({ left: halfWidth, top: 0, width: width - halfWidth, height })
          .toBuffer();
        
        // Add the image halves to the composite array
        const compositeArray = [
          { input: leftHalf, left: 0, top: 0 },
          { input: rightHalf, left: halfWidth, top: 0 }
        ];
        
        // Add labels if requested
        if (splitConfig.addLabels) {
          // Create light mode label
          const lightLabelBuffer = await this.createLabel('LIGHT MODE', 'light');
          compositeArray.push({ input: lightLabelBuffer, left: 20, top: 20 });
          
          // Create dark mode label
          const darkLabelBuffer = await this.createLabel('DARK MODE', 'dark');
          compositeArray.push({ input: darkLabelBuffer, left: halfWidth + 20, top: 20 });
        }
        
        // Add icons if requested
        if (splitConfig.addIcons) {
          // Create light mode icon
          const lightIconBuffer = await this.createThemeIcon('light');
          compositeArray.push({ input: lightIconBuffer, left: 20, top: 70 });
          
          // Create dark mode icon
          const darkIconBuffer = await this.createThemeIcon('dark');
          compositeArray.push({ input: darkIconBuffer, left: halfWidth + 20, top: 70 });
        }
        
        // Create a new image with both halves and overlays
        await sharp({
          create: {
            width,
            height,
            channels: 4,
            background: { r: 0, g: 0, b: 0, alpha: 0 }
          }
        })
          .composite(compositeArray)
          .toFile(outputPath);
      } else if (splitConfig.type === 'diagonal') {
        try {
          // Get the dimensions from the metadata
          const { width = 1920, height = 1080 } = metadata;
          
          // Load both images and resize them to the same dimensions
          const lightImg = await sharp(lightBuffer).resize(width, height).toBuffer();
          const darkImg = await sharp(darkBuffer).resize(width, height).toBuffer();
          
          // Create a simple diagonal mask from top-left to bottom-right
          const maskPath = path.join(os.tmpdir(), `${outputName}-mask.png`);
          const svgBuffer = Buffer.from(`
            <svg width="${width}" height="${height}">
              <polygon points="0,0 ${width},0 0,${height}" fill="white" />
            </svg>
          `);
          
          await sharp(svgBuffer)
            .toFile(maskPath);
          
          // Create an inverted mask
          const invertedMaskPath = path.join(os.tmpdir(), `${outputName}-inverted-mask.png`);
          const invertedSvgBuffer = Buffer.from(`
            <svg width="${width}" height="${height}">
              <polygon points="${width},0 ${width},${height} 0,${height}" fill="white" />
            </svg>
          `);
          
          await sharp(invertedSvgBuffer)
            .toFile(invertedMaskPath);
          
          // Apply the mask to the light image
          const maskedLightPath = path.join(os.tmpdir(), `${outputName}-masked-light.png`);
          await sharp(lightImg)
            .composite([
              {
                input: maskPath,
                blend: 'dest-in'
              }
            ])
            .toFile(maskedLightPath);
          
          // Apply the inverted mask to the dark image
          const maskedDarkPath = path.join(os.tmpdir(), `${outputName}-masked-dark.png`);
          await sharp(darkImg)
            .composite([
              {
                input: invertedMaskPath,
                blend: 'dest-in'
              }
            ])
            .toFile(maskedDarkPath);
          
          // Combine the masked images
          await sharp({
            create: {
              width,
              height,
              channels: 4,
              background: { r: 0, g: 0, b: 0, alpha: 0 }
            }
          })
            .composite([
              { input: maskedLightPath, blend: 'over' },
              { input: maskedDarkPath, blend: 'over' }
            ])
            .toFile(outputPath);
          
          // Clean up temporary files
          try {
            await fs.promises.unlink(maskPath);
            await fs.promises.unlink(invertedMaskPath);
            await fs.promises.unlink(maskedLightPath);
            await fs.promises.unlink(maskedDarkPath);
          } catch (cleanupError) {
            logger.warn(`Failed to clean up temporary files: ${cleanupError}`);
          }
          
          logger.info(`Split view image saved to ${outputPath}`);
          
          // Clean up individual screenshots if requested
          if (splitConfig.cleanupIndividualScreenshots) {
            try {
              await fs.promises.unlink(lightImagePath);
              await fs.promises.unlink(darkImagePath);
              logger.info(`Cleaned up individual screenshots for ${outputName}`);
            } catch (cleanupError) {
              logger.warn(`Failed to clean up individual screenshots: ${cleanupError}`);
            }
          }
        } catch (error) {
          logger.error(`Error creating diagonal split view: ${error}`);
          logger.info(`Falling back to vertical split for ${outputName}`);
          
          // Fall back to vertical split
          const halfWidth = Math.floor(width / 2);
          
          // Extract left half from light image
          const leftHalf = await sharp(lightBuffer)
            .extract({ left: 0, top: 0, width: halfWidth, height })
            .toBuffer();
          
          // Extract right half from dark image
          const rightHalf = await sharp(darkBuffer)
            .extract({ left: halfWidth, top: 0, width: width - halfWidth, height })
            .toBuffer();
          
          // Add the image halves to the composite array
          const compositeArray = [
            { input: leftHalf, left: 0, top: 0 },
            { input: rightHalf, left: halfWidth, top: 0 }
          ];
          
          // Create a new image with both halves
          await sharp({
            create: {
              width,
              height,
              channels: 4,
              background: { r: 0, g: 0, b: 0, alpha: 0 }
            }
          })
            .composite(compositeArray)
            .toFile(outputPath);
          
          logger.info(`Using vertical split instead of diagonal for ${outputName} (fallback)`);
          
          // Clean up individual screenshots if requested
          if (splitConfig.cleanupIndividualScreenshots) {
            try {
              await fs.promises.unlink(lightImagePath);
              await fs.promises.unlink(darkImagePath);
              logger.info(`Cleaned up individual screenshots for ${outputName}`);
            } catch (cleanupError) {
              logger.warn(`Failed to clean up individual screenshots: ${cleanupError}`);
            }
          }
        }
      } else {
        throw new Error(`Unsupported split type: ${splitConfig.type}`);
      }
      
      return outputPath;
    } catch (error) {
      logger.error(`Error creating split view: ${error}`);
      throw error;
    }
  }
  
  // Helper method to create a text label
  private async createLabel(text: string, theme: 'light' | 'dark'): Promise<Buffer> {
    // Create a text overlay with a solid background
    const svgBuffer = Buffer.from(`
      <svg width="200" height="40">
        <rect x="0" y="0" width="200" height="40" rx="5" ry="5" 
              fill="${theme === 'light' ? 'rgba(0,0,0,1)' : 'rgba(255,255,255,1)'}" />
        <text x="100" y="25" font-family="Arial" font-size="16" font-weight="bold" 
              text-anchor="middle" fill="${theme === 'light' ? 'white' : 'black'}">${text}</text>
      </svg>
    `);
    
    return await sharp(svgBuffer).toBuffer();
  }
  
  // Helper method to create a theme icon
  private async createThemeIcon(theme: 'light' | 'dark'): Promise<Buffer> {
    // Create a simple sun or moon icon with solid background
    const svgBuffer = Buffer.from(`
      <svg width="40" height="40">
        <rect x="0" y="0" width="40" height="40" rx="20" ry="20" 
              fill="${theme === 'light' ? 'rgba(0,0,0,1)' : 'rgba(255,255,255,1)'}" />
        ${theme === 'light' 
          ? '<circle cx="20" cy="20" r="12" fill="rgba(255,200,0,1)" stroke="rgba(255,255,255,0.5)" stroke-width="1"/>' 
          : '<circle cx="20" cy="20" r="12" fill="rgba(100,100,200,1)" stroke="rgba(0,0,0,0.5)" stroke-width="1"/><circle cx="15" cy="15" r="4" fill="rgba(50,50,100,1)"/>'}
      </svg>
    `);
    
    return await sharp(svgBuffer).toBuffer();
  }
  
  /**
   * Set up mock data for testing
   */
  private async setupMockData(page: Page): Promise<void> {
    logger.info('Setting up server-side mock data');
    
    // Set mock data flags in localStorage before navigation
    await page.evaluateOnNewDocument(() => {
      localStorage.setItem('use_mock_data', 'true');
      localStorage.setItem('MOCK_DATA_ENABLED', 'true');
      
      // Remove client-side mock data settings
      localStorage.removeItem('mock_enabled');
      localStorage.removeItem('MOCK_SERVER_URL');
      localStorage.removeItem('MOCK_DATA');
    });
    
    // Execute any custom setup script from config
    if (this.config.mockData?.setupScript) {
      await page.evaluateOnNewDocument(this.config.mockData.setupScript);
    }
  }
  
  async captureAllScreenshots(): Promise<void> {
    try {
      await this.initialize();
      
      for (const screenshot of this.config.screenshots) {
        const { path: pagePath, name, viewportSize, waitForSelector, cropRegion } = screenshot;
        const enableFilters = screenshot.enableFilters !== false; // Default to true if not specified
        
        // For split views, we need to ensure both light and dark screenshots have the same filter state
        if (screenshot.createSplitView) {
          // Create a new browser page for consistent filter state
          if (!this.browser) {
            throw new Error('Browser not initialized');
          }
          
          const setupPage = await this.browser.newPage();
          
          try {
            // Set viewport size
            await setupPage.setViewport(viewportSize || { width: 1440, height: 900 });
            
            // Setup mock data if configured
            if (this.config.mockData?.enabled) {
              await this.setupMockData(setupPage);
            }
            
            // Navigate to the page
            const url = `${this.baseUrl}${pagePath}`;
            logger.info(`Setting up filter state at ${url}`);
            await setupPage.goto(url, { waitUntil: 'networkidle2' });
            
            // Wait for specific element if needed
            if (waitForSelector) {
              await setupPage.waitForSelector(waitForSelector, { visible: true });
            }
            
            // Wait for page to stabilize
            await setupPage.evaluate(() => new Promise(resolve => setTimeout(resolve, 2000)));
            
            // Toggle filters to desired state and save to localStorage
            if (enableFilters) {
              // Toggle filters on and save state to localStorage
              await setupPage.evaluate(() => {
                // Save filter state to localStorage so it persists across page loads
                localStorage.setItem('app_filter_state', JSON.stringify({
                  filtersEnabled: true,
                  timestamp: Date.now()
                }));
              });
              
              // Apply filters
              await this.toggleFilters(setupPage, true);
              logger.info('Filter state set to ENABLED for both themes');
            } else {
              // Ensure filters are off and save state to localStorage
              await setupPage.evaluate(() => {
                // Save filter state to localStorage so it persists across page loads
                localStorage.setItem('app_filter_state', JSON.stringify({
                  filtersEnabled: false,
                  timestamp: Date.now()
                }));
              });
              logger.info('Filter state set to DISABLED for both themes');
            }
            
            // Wait for filters to apply
            await setupPage.evaluate(() => new Promise(resolve => setTimeout(resolve, 1000)));
          } finally {
            // Close the setup page
            await setupPage.close();
          }
          
          // Take light mode screenshot with consistent filter state
          const lightPath = await this.takeScreenshot(
            pagePath,
            `${name}-light`,
            viewportSize,
            'light',
            waitForSelector,
            cropRegion,
            false, // Don't toggle filters again, use the state we just set
            screenshot.beforeScreenshot
          );
          
          // Take dark mode screenshot with consistent filter state
          const darkPath = await this.takeScreenshot(
            pagePath,
            `${name}-dark`,
            viewportSize,
            'dark',
            waitForSelector,
            cropRegion,
            false, // Don't toggle filters again, use the state we just set
            screenshot.beforeScreenshot
          );
          
          // Create split view
          await this.createSplitView(
            lightPath,
            darkPath,
            name,
            {
              type: 'diagonal',
              ...screenshot.splitViewConfig,
              cleanupIndividualScreenshots: screenshot.cleanupIndividualScreenshots
            }
          );
        } else {
          // For non-split view screenshots, just take them normally
          // Take light mode screenshot if not darkModeOnly
          if (!screenshot.darkModeOnly) {
            await this.takeScreenshot(
              pagePath,
              `${name}${screenshot.lightModeOnly ? '' : '-light'}`,
              viewportSize,
              'light',
              waitForSelector,
              cropRegion,
              enableFilters,
              screenshot.beforeScreenshot
            );
          }
          
          // Take dark mode screenshot if not lightModeOnly
          if (!screenshot.lightModeOnly) {
            await this.takeScreenshot(
              pagePath,
              `${name}${screenshot.darkModeOnly ? '' : '-dark'}`,
              viewportSize,
              'dark',
              waitForSelector,
              cropRegion,
              enableFilters,
              screenshot.beforeScreenshot
            );
          }
        }
      }
    } catch (error) {
      logger.error(`Error capturing screenshots: ${error}`);
      throw error;
    } finally {
      await this.close();
    }
  }
}

export default ScreenshotTool; 