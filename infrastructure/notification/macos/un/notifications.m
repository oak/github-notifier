// notifications.m — UNUserNotificationCenter delegate for GitHub Notifier.
// Sends notifications and opens URLs when the user clicks them.
//
// Build constraints: darwin only (enforced by the Go file's //go:build tag).

#import <Foundation/Foundation.h>
#import <AppKit/AppKit.h>
#import <UserNotifications/UserNotifications.h>

// ---- Delegate ---------------------------------------------------------------

@interface GHNDelegate : NSObject <UNUserNotificationCenterDelegate>
@end

@implementation GHNDelegate

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
       willPresentNotification:(UNNotification *)notification
         withCompletionHandler:(void (^)(UNNotificationPresentationOptions))completionHandler {
    // Show banner + sound even when the app is in the foreground
    completionHandler(UNNotificationPresentationOptionBanner |
                      UNNotificationPresentationOptionSound);
}

- (void)userNotificationCenter:(UNUserNotificationCenter *)center
didReceiveNotificationResponse:(UNNotificationResponse *)response
         withCompletionHandler:(void (^)(void))completionHandler {
    NSString *urlString = response.notification.request.content.userInfo[@"open_url"];
    if (urlString.length > 0) {
        NSURL *url = [NSURL URLWithString:urlString];
        if (url) {
            [[NSWorkspace sharedWorkspace] openURL:url];
        }
    }
    completionHandler();
}

@end

// ---- C interface ------------------------------------------------------------

static GHNDelegate *gDelegate = nil;

// ghn_setup must be called once from the main thread before any notification
// is sent. It registers the delegate and requests authorisation.
// ghn_available returns 1 if the process was launched from a proper .app bundle
// (i.e. has a bundleProxy registered with launchd). Patching the info dictionary
// is not sufficient — UNUserNotificationCenter requires the real registration.
int ghn_available(void) {
    // A real .app bundle path ends with ".app/Contents/MacOS/<binary>".
    // A bare binary or test binary runs from a temp directory.
    NSString *path = [[NSBundle mainBundle] bundlePath];
    return [path hasSuffix:@".app"] ? 1 : 0;
}

void ghn_setup(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        UNUserNotificationCenter *center = [UNUserNotificationCenter currentNotificationCenter];
        gDelegate = [[GHNDelegate alloc] init];
        center.delegate = gDelegate;

        [center requestAuthorizationWithOptions:(UNAuthorizationOptionAlert |
                                                 UNAuthorizationOptionSound)
                              completionHandler:^(BOOL granted, NSError *error) {
            if (!granted) {
                NSLog(@"[github-notifier] Notification permission denied: %@", error);
            }
        }];
    });
}

// ghn_send delivers a single notification.
//   identifier  — unique ID (used for deduplication / replacement)
//   title       — bold header line
//   body        — detail text
//   openURL     — if non-empty, opened in the default browser when clicked
void ghn_send(const char *identifier, const char *title, const char *body, const char *openURL) {
    NSString *idStr    = [NSString stringWithUTF8String:identifier];
    NSString *titleStr = [NSString stringWithUTF8String:title];
    NSString *bodyStr  = [NSString stringWithUTF8String:body];
    NSString *urlStr   = openURL && strlen(openURL) > 0
                            ? [NSString stringWithUTF8String:openURL]
                            : nil;

    dispatch_async(dispatch_get_main_queue(), ^{
        UNMutableNotificationContent *content = [[UNMutableNotificationContent alloc] init];
        content.title = titleStr;
        content.body  = bodyStr;
        content.sound = [UNNotificationSound defaultSound];
        if (urlStr) {
            content.userInfo = @{ @"open_url": urlStr };
        }

        UNTimeIntervalNotificationTrigger *trigger =
            [UNTimeIntervalNotificationTrigger triggerWithTimeInterval:0.1 repeats:NO];

        UNNotificationRequest *request =
            [UNNotificationRequest requestWithIdentifier:idStr
                                                 content:content
                                                 trigger:trigger];

        [[UNUserNotificationCenter currentNotificationCenter]
            addNotificationRequest:request
             withCompletionHandler:^(NSError *error) {
                if (error) {
                    NSLog(@"[github-notifier] Failed to send notification: %@", error);
                }
            }];
    });
}
