package com.olivierh.kickoff2;

import java.io.File;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.InterruptedIOException;
import java.util.Locale;

/** Bounded private copies of documents explicitly selected by the user. */
final class DocumentImport {
    private DocumentImport() {}

    static File copy(InputStream source, File directory, String displayName) throws IOException {
        return copy(source, directory, displayName, false);
    }

    /** A replay selection set preserves names used by its original .rep index. */
    static File copyNamed(InputStream source, File directory, String displayName) throws IOException {
        return copy(source, directory, displayName, true);
    }

    private static File copy(InputStream source, File directory, String displayName, boolean named) throws IOException {
        if (source == null) {
            throw new IOException("The selected document could not be opened.");
        }
        String name = displayName == null ? "" : displayName;
        int dot = name.lastIndexOf('.');
        String extension = dot < 0 ? "" : name.substring(dot).toLowerCase(Locale.ROOT);
        if (!extension.equals(".sqd") && !extension.equals(".tac") && !extension.equals(".adf")
                && !extension.equals(".lge") && !extension.equals(".cup")
                && !extension.equals(".grp") && !extension.equals(".rep") && !extension.matches("\\.z[0-9]{1,5}")) {
            throw new IOException("Select an original squad, tactic, competition, replay or data ADF.");
        }
        long limit = extension.equals(".adf") ? 32L * 1024 * 1024 : 64L * 1024;
        String stem = name.substring(0, dot).replaceAll("[^A-Za-z0-9_-]", "_");
        if (stem.isEmpty()) {
            stem = "original";
        }
        stem = stem.substring(0, Math.min(stem.length(), 64));
        if (!directory.isDirectory() && !directory.mkdirs()) {
            throw new IOException("The private import folder could not be created.");
        }
        // Atomic creation gives every selection a unique name, preserving prior imports.
        File target;
        if (named) {
            if (name.isEmpty() || name.length() > 128 || name.indexOf('/') >= 0 || name.indexOf('\\') >= 0
                    || name.indexOf('\0') >= 0 || name.equals(".") || name.equals("..")) {
                throw new IOException("Invalid original filename.");
            }
            target = new File(directory, name);
            if (!target.createNewFile()) {
                throw new IOException("The selection contains a duplicate filename.");
            }
        } else {
            target = File.createTempFile(stem + "-import-", extension, directory);
        }
        boolean complete = false;
        try {
            try (FileOutputStream output = new FileOutputStream(target)) {
                byte[] buffer = new byte[16384];
                long total = 0;
                int count;
                while ((count = source.read(buffer)) != -1) {
                    if (Thread.currentThread().isInterrupted()) {
                        throw new InterruptedIOException("The import was interrupted.");
                    }
                    total += count;
                    if (total > limit) {
                        throw new IOException(extension.equals(".adf")
                                ? "The data disk exceeds 32 MiB."
                                : "The original data file exceeds 64 KiB.");
                    }
                    output.write(buffer, 0, count);
                }
            }
            complete = true;
            return target;
        } finally {
            if (!complete) {
                target.delete();
            }
        }
    }
}
