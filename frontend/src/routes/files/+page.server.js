export const load = async () => {
  const response = await fetch(`http://localhost:37999/list/`);
  const responseText = await response.text();

  return {
      data: responseText
  };
};