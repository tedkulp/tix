// ChatGPT
#[allow(dead_code)]
pub fn truncate_and_dash_case(input: &str, max_length: usize) -> String {
    if max_length == 0 {
        return String::new();
    }

    // Truncate at max_length
    let truncated = if input.len() > max_length {
        let slice = &input[..max_length];
        // Find the last space within the truncated portion
        match slice.rfind(' ') {
            Some(last_space_index) => &slice[..last_space_index],
            None => slice, // No spaces found, use the full truncated slice
        }
    } else {
        input
    };

    // Convert to dash case
    truncated
        .trim() // Remove leading and trailing whitespace
        .to_lowercase() // Convert to lowercase
        .chars()
        .filter(|c| c.is_alphanumeric() || c.is_whitespace()) // Keep alphanumeric and spaces
        .collect::<String>()
        .split_whitespace() // Split by whitespace
        .collect::<Vec<&str>>() // Collect into a vector
        .join("-") // Join with dashes
}
