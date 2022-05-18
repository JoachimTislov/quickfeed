import { By } from 'selenium-webdriver'
import { isOverlapping, setupDrivers } from '../testHelpers/testHelpers'

describe("Front page elements login and logo should not overlap", () => {
    // Laptop, desktop, mobile
    const overlapTests: { width: number, height: number, want: boolean }[] = [
        // Insert which resolutions to test here
        { width: 1920, height: 1080, want: false }, // Desktop
        { width: 1366, height: 768, want: false }, // Laptop
        { width: 960, height: 1080, want: false }, // Split screen desktop
        { width: 683, height: 768, want: false } // Split screen laptop
    ]

    const drivers = setupDrivers()

    drivers.forEach(driver => {
        overlapTests.forEach(test => {
            it(`Should not overlap on res ${test.width}x${test.height}`, async () => {
                await driver.manage().window().setRect({ width: test.width, height: test.height })

                const logo = await driver.findElement(By.className("navbar-brand"))
                const signIn = await driver.findElement(By.className("signIn"))
                const rect = await logo.getRect()
                const rect2 = await signIn.getRect()

                const overlap = isOverlapping(rect, rect2)

                jest.setTimeout(50000)
                expect(overlap).toBe(test.want)
            }, 50000)
        })
    })
})
