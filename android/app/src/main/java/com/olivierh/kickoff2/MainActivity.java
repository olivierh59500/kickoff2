package com.olivierh.kickoff2;

import android.app.Activity;
import android.content.Intent;
import android.content.ClipData;
import android.database.Cursor;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.provider.OpenableColumns;
import android.view.InputDevice;
import android.view.KeyEvent;
import android.view.View;
import android.view.WindowInsets;
import android.view.WindowInsetsController;
import android.view.WindowManager;
import android.window.OnBackInvokedDispatcher;

import java.io.File;
import java.io.IOException;
import java.io.InputStream;
import java.io.FileInputStream;
import java.io.OutputStream;
import java.util.ArrayList;
import java.util.List;
import java.util.Locale;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

import com.olivierh.kickoff2.mobile.EbitenView;
import com.olivierh.kickoff2.mobile.Mobile;

/** Android lifecycle bridge; gameplay, audio, rendering and touch input stay in Go. */
public final class MainActivity extends Activity {
    private static final int OPEN_ORIGINAL_DOCUMENT = 1;
    private static final int CREATE_REPLAY_DOCUMENT = 2;
    private String pendingReplayExport;
    private EbitenView ebitenView;
    private boolean documentPickerActive;
    private boolean documentCopyActive;
    private final Handler platformHandler = new Handler(Looper.getMainLooper());
    private final ExecutorService documentWorker = Executors.newSingleThreadExecutor();
    private final Runnable pollPlatformRequests = new Runnable() {
        @Override
        public void run() {
            if (!documentPickerActive && !documentCopyActive) {
                String export = Mobile.takeReplayExportRequest();
                if (!export.isEmpty()) {
                    openReplayExport(export);
                } else if (Mobile.takeImportPickerRequest()) {
                    openOriginalDocument();
                }
            }
            if (!isFinishing()) {
                platformHandler.postDelayed(this, 100);
            }
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        // The host waits for the view update loop before creating Go graphics or
        // audio resources. EbitenView registers the JNI context itself.
        Mobile.configure(getFilesDir().getAbsolutePath());

        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            WindowManager.LayoutParams attributes = getWindow().getAttributes();
            attributes.layoutInDisplayCutoutMode =
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES;
            getWindow().setAttributes(attributes);
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            getOnBackInvokedDispatcher().registerOnBackInvokedCallback(
                    OnBackInvokedDispatcher.PRIORITY_DEFAULT, this::handleBack);
        }

        ebitenView = new EbitenView(this);
        ebitenView.setFocusableInTouchMode(true);
        ebitenView.requestFocus();
        setContentView(ebitenView);
        hideSystemUi();
    }

    @Override
    protected void onPause() {
        platformHandler.removeCallbacks(pollPlatformRequests);
        if (ebitenView != null) {
            // Queue the pause on the game thread before suspending its surface.
            Mobile.cancelInput();
            Mobile.requestPause();
            ebitenView.suspendGame();
        }
        super.onPause();
    }

    @Override
    protected void onResume() {
        super.onResume();
        hideSystemUi();
        if (ebitenView != null) {
            ebitenView.resumeGame();
        }
        platformHandler.removeCallbacks(pollPlatformRequests);
        platformHandler.post(pollPlatformRequests);
    }

    @Override
    protected void onDestroy() {
        platformHandler.removeCallbacks(pollPlatformRequests);
        documentWorker.shutdownNow();
        super.onDestroy();
    }

    private void openOriginalDocument() {
        Intent request = new Intent(Intent.ACTION_OPEN_DOCUMENT);
        request.addCategory(Intent.CATEGORY_OPENABLE);
        request.setType("*/*");
        request.putExtra(Intent.EXTRA_ALLOW_MULTIPLE, true);
        request.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION);
        documentPickerActive = true;
        try {
            startActivityForResult(request, OPEN_ORIGINAL_DOCUMENT);
        } catch (RuntimeException failure) {
            documentPickerActive = false;
            Mobile.importCompleted("DOCUMENT PICKER UNAVAILABLE");
        }
    }

    @Override
    protected void onActivityResult(int requestCode, int resultCode, Intent data) {
        super.onActivityResult(requestCode, resultCode, data);
        if (requestCode == CREATE_REPLAY_DOCUMENT) {
            documentPickerActive = false;
            String source = pendingReplayExport;
            pendingReplayExport = null;
            if (resultCode != RESULT_OK || data == null || data.getData() == null || source == null) {
                Mobile.exportCompleted("EXPORT CANCELLED");
                return;
            }
            Uri destination = data.getData();
            documentCopyActive = true;
            documentWorker.execute(() -> copyReplayExport(source, destination));
            return;
        }
        if (requestCode != OPEN_ORIGINAL_DOCUMENT) {
            return;
        }
        documentPickerActive = false;
        if (resultCode != RESULT_OK || data == null) {
            Mobile.importCompleted("IMPORT CANCELLED");
            return;
        }
        List<Uri> documents = new ArrayList<>();
        ClipData selected = data.getClipData();
        if (selected != null) {
            if (selected.getItemCount() > 64) {
                Mobile.importCompleted("SELECT AT MOST 64 ORIGINAL FILES");
                return;
            }
            for (int index = 0; index < selected.getItemCount() && index < 64; index++) {
                documents.add(selected.getItemAt(index).getUri());
            }
        } else if (data.getData() != null) {
            documents.add(data.getData());
        }
        if (documents.isEmpty()) {
            Mobile.importCompleted("IMPORT CANCELLED");
            return;
        }
        documentCopyActive = true;
        documentWorker.execute(() -> copyOriginalDocuments(documents));
    }

    private void copyOriginalDocuments(List<Uri> documents) {
        List<File> copied = new ArrayList<>();
        File batch = null;
        String message;
        try {
            File imports = new File(getFilesDir(), "imports");
            for (Uri document : documents) {
                String displayName = document.getLastPathSegment();
                try (Cursor cursor = getContentResolver().query(document,
                        new String[]{OpenableColumns.DISPLAY_NAME}, null, null, null)) {
                    if (cursor != null && cursor.moveToFirst() && !cursor.isNull(0)) {
                        displayName = cursor.getString(0);
                    }
                }
                String lower = displayName == null ? "" : displayName.toLowerCase(Locale.ROOT);
                boolean replay = lower.endsWith(".rep") || lower.endsWith(".grp") || lower.matches(".*\\.z[0-9]{1,5}");
                if (replay && batch == null) {
                    if (!imports.isDirectory() && !imports.mkdirs()) {
                        throw new IOException("The private import folder could not be created.");
                    }
                    batch = File.createTempFile("replay-set-", "", imports);
                    if (!batch.delete() || !batch.mkdir()) {
                        throw new IOException("The replay selection folder could not be created.");
                    }
                }
                try (InputStream source = getContentResolver().openInputStream(document)) {
                    copied.add(replay ? DocumentImport.copyNamed(source, batch, displayName)
                            : DocumentImport.copy(source, imports, displayName));
                }
            }
            // Validate indices after all companions have reached the same folder.
            for (File file : copied) {
                String validation = Mobile.validateImportedFile(file.getAbsolutePath());
                if (!validation.isEmpty()) {
                    throw new IOException(validation);
                }
            }
            message = "ORIGINAL FILES READY";
        } catch (IOException | RuntimeException failure) {
            for (File file : copied) {
                file.delete();
            }
            if (batch != null) {
                batch.delete();
            }
            message = "IMPORT FAILED: " + failure.getMessage();
        }
        // Keep document contents and provider URIs out of Android logs.
        Mobile.importCompleted(message);
        runOnUiThread(() -> documentCopyActive = false);
    }

    private void openReplayExport(String filename) {
        Intent request = new Intent(Intent.ACTION_CREATE_DOCUMENT);
        request.addCategory(Intent.CATEGORY_OPENABLE);
        request.setType("application/octet-stream");
        request.putExtra(Intent.EXTRA_TITLE, new File(filename).getName());
        request.addFlags(Intent.FLAG_GRANT_WRITE_URI_PERMISSION);
        pendingReplayExport = filename;
        documentPickerActive = true;
        try {
            startActivityForResult(request, CREATE_REPLAY_DOCUMENT);
        } catch (RuntimeException failure) {
            pendingReplayExport = null;
            documentPickerActive = false;
            Mobile.exportCompleted("DOCUMENT PICKER UNAVAILABLE");
        }
    }

    private void copyReplayExport(String filename, Uri destination) {
        String message;
        try (InputStream source = new FileInputStream(filename);
                OutputStream output = getContentResolver().openOutputStream(destination, "wt")) {
            if (output == null) {
                throw new IOException("The export document could not be opened.");
            }
            byte[] buffer = new byte[16384];
            int count;
            while ((count = source.read(buffer)) != -1) {
                output.write(buffer, 0, count);
            }
            String lower = filename.toLowerCase(java.util.Locale.ROOT);
            message = lower.endsWith(".lge") || lower.endsWith(".cup")
                    ? "ORIGINAL COMPETITION EXPORTED" : "ORIGINAL REPLAY EXPORTED";
        } catch (IOException | RuntimeException failure) {
            message = "EXPORT FAILED: " + failure.getMessage();
        }
        Mobile.exportCompleted(message);
        runOnUiThread(() -> documentCopyActive = false);
    }

    @Override
    public void onBackPressed() {
        handleBack();
    }

    @Override
    public boolean dispatchKeyEvent(KeyEvent event) {
        // EbitenView consumes all legacy key events. Route the older Android
        // system Back key before it reaches the view, preserving gamepad input.
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU
                && event.getKeyCode() == KeyEvent.KEYCODE_BACK
                && !event.isFromSource(InputDevice.SOURCE_GAMEPAD)
                && !event.isFromSource(InputDevice.SOURCE_JOYSTICK)) {
            if (event.getAction() == KeyEvent.ACTION_UP && !event.isCanceled()) {
                handleBack();
            }
            return true;
        }
        return super.dispatchKeyEvent(event);
    }

    private void handleBack() {
        // The Go application handles pausing and returning to its title screen;
        // Android closes the activity once that title screen is already visible.
        if (Mobile.isAtTitle()) {
            finish();
        } else {
            Mobile.requestBack();
        }
    }

    @Override
    public void onWindowFocusChanged(boolean hasFocus) {
        super.onWindowFocusChanged(hasFocus);
        if (hasFocus) {
            hideSystemUi();
        } else {
            Mobile.cancelInput();
        }
    }

    private void hideSystemUi() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            getWindow().setDecorFitsSystemWindows(false);
            WindowInsetsController controller =
                    getWindow().getDecorView().getWindowInsetsController();
            if (controller != null) {
                controller.hide(WindowInsets.Type.statusBars() | WindowInsets.Type.navigationBars());
                controller.setSystemBarsBehavior(
                        WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE);
            }
            return;
        }
        getWindow().getDecorView().setSystemUiVisibility(
                View.SYSTEM_UI_FLAG_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
                        | View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN
                        | View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION
                        | View.SYSTEM_UI_FLAG_LAYOUT_STABLE);
    }
}
