// Native macOS tray implementation using NSStatusItem + NSAttributedString.

#import <Cocoa/Cocoa.h>
#include "_cgo_export.h"

static NSStatusItem *statusItem = nil;
static NSMenu *menu = nil;

// Menu item tags for updating titles
enum {
    TAG_STATUS = 1,
    TAG_5H,
    TAG_RESET_5H,
    TAG_7D,
    TAG_RESET_7D,
    TAG_AUTH,
    TAG_ERROR,
    TAG_REFRESH,
    TAG_QUIT,
};

@interface TrayDelegate : NSObject <NSApplicationDelegate>
@end

@implementation TrayDelegate

- (void)refreshClicked:(id)sender {
    goRefreshClicked();
}

- (void)quitClicked:(id)sender {
    goQuitClicked();
}

@end

static TrayDelegate *delegate = nil;

void initTray(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];

        delegate = [[TrayDelegate alloc] init];

        statusItem = [[NSStatusBar systemStatusBar]
            statusItemWithLength:NSVariableStatusItemLength];

        menu = [[NSMenu alloc] init];

        // Status item
        NSMenuItem *statusMenuItem = [[NSMenuItem alloc] initWithTitle:@"Status: idle"
            action:nil keyEquivalent:@""];
        [statusMenuItem setTag:TAG_STATUS];
        [statusMenuItem setEnabled:NO];
        [menu addItem:statusMenuItem];

        [menu addItem:[NSMenuItem separatorItem]];

        // 5h detail
        NSMenuItem *item5h = [[NSMenuItem alloc] initWithTitle:@"5h: --"
            action:nil keyEquivalent:@""];
        [item5h setTag:TAG_5H];
        [item5h setEnabled:NO];
        [menu addItem:item5h];

        // 5h reset
        NSMenuItem *reset5h = [[NSMenuItem alloc] initWithTitle:@"  resets in —"
            action:nil keyEquivalent:@""];
        [reset5h setTag:TAG_RESET_5H];
        [reset5h setEnabled:NO];
        [menu addItem:reset5h];

        // 7d detail
        NSMenuItem *item7d = [[NSMenuItem alloc] initWithTitle:@"7d: --"
            action:nil keyEquivalent:@""];
        [item7d setTag:TAG_7D];
        [item7d setEnabled:NO];
        [menu addItem:item7d];

        // 7d reset
        NSMenuItem *reset7d = [[NSMenuItem alloc] initWithTitle:@"  resets in —"
            action:nil keyEquivalent:@""];
        [reset7d setTag:TAG_RESET_7D];
        [reset7d setEnabled:NO];
        [menu addItem:reset7d];

        // Auth (hidden by default)
        NSMenuItem *authItem = [[NSMenuItem alloc] initWithTitle:@""
            action:nil keyEquivalent:@""];
        [authItem setTag:TAG_AUTH];
        [authItem setEnabled:NO];
        [authItem setHidden:YES];
        [menu addItem:authItem];

        // Error (hidden by default)
        NSMenuItem *errorItem = [[NSMenuItem alloc] initWithTitle:@""
            action:nil keyEquivalent:@""];
        [errorItem setTag:TAG_ERROR];
        [errorItem setEnabled:NO];
        [errorItem setHidden:YES];
        [menu addItem:errorItem];

        [menu addItem:[NSMenuItem separatorItem]];

        // Refresh
        NSMenuItem *refreshItem = [[NSMenuItem alloc] initWithTitle:@"Refresh now"
            action:@selector(refreshClicked:) keyEquivalent:@""];
        [refreshItem setTag:TAG_REFRESH];
        [refreshItem setTarget:delegate];
        [menu addItem:refreshItem];

        // Quit
        NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit"
            action:@selector(quitClicked:) keyEquivalent:@"q"];
        [quitItem setTag:TAG_QUIT];
        [quitItem setTarget:delegate];
        [menu addItem:quitItem];

        [statusItem setMenu:menu];
    }
}

void runApp(void) {
    @autoreleasepool {
        [NSApp run];
    }
}

void stopApp(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp terminate:nil];
    });
}

void setTrayTitle(const char *title, double r, double g, double b) {
    @autoreleasepool {
        NSString *str = [NSString stringWithUTF8String:title];
        NSColor *color = [NSColor colorWithCalibratedRed:r green:g blue:b alpha:1.0];
        NSDictionary *attrs = @{
            NSForegroundColorAttributeName: color,
            NSFontAttributeName: [NSFont menuBarFontOfSize:0],
        };
        NSAttributedString *attrStr = [[NSAttributedString alloc]
            initWithString:str attributes:attrs];

        dispatch_async(dispatch_get_main_queue(), ^{
            NSStatusBarButton *btn = [statusItem button];
            if (btn) {
                [btn setAttributedTitle:attrStr];
            }
        });
    }
}

void setMenuItemTitle(int tag, const char *title) {
    @autoreleasepool {
        NSString *str = [NSString stringWithUTF8String:title];
        dispatch_async(dispatch_get_main_queue(), ^{
            NSMenuItem *item = [menu itemWithTag:tag];
            if (item) {
                [item setTitle:str];
            }
        });
    }
}

void setMenuItemHidden(int tag, int hidden) {
    @autoreleasepool {
        dispatch_async(dispatch_get_main_queue(), ^{
            NSMenuItem *item = [menu itemWithTag:tag];
            if (item) {
                [item setHidden:(hidden != 0)];
            }
        });
    }
}
